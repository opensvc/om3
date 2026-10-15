package object

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/statusbus"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

// Provision allocates and starts the local instance of the object
//
// The provision, and the stop ending it off the placement leader, report to
// the collector as one provision.
func (t *actor) Provision(ctx context.Context) (err error) {
	if err := t.validateAction(); err != nil {
		return err
	}
	provision := t.config.GetBool(key.New("", "provision"))
	if !provision {
		return fmt.Errorf("provision is disabled: make sure all resources have been provisioned by a sysadmin and execute 'instance provision --state-only")
	}
	ctx2 := actioncontext.WithProps(ctx, actioncontext.Provision)
	ctx2, done := t.beginCollectorAction(ctx2, "provision")
	defer func() { done(err, recover()) }()
	ctx = withCollectorActionOf(ctx, ctx2)
	t.setenv("provision", actioncontext.IsLeader(ctx2))
	unlock, err := t.lockAction(ctx2)
	if err != nil {
		return err
	}
	defer unlock()

	// The preflight checks that nothing on the node holds what the
	// provision would take, before anything is provisioned: alone, as the
	// daemon has every node do before the leader provisions an object
	// provisioned nowhere, and before a provision as leader, unless the
	// provision adopts what it finds.
	if actioncontext.IsPreflight(ctx2) {
		return t.lockedProvisionPreflight(ctx2)
	}
	if actioncontext.IsLeader(ctx2) && !actioncontext.IsAdopt(ctx2) {
		if err := t.lockedProvisionPreflight(ctx2); err != nil {
			return err
		}
	}

	// Off the placement leader, and unless --disable-rollback is set, a
	// provision leaves the instance as it found it: an instance found down
	// is left down for the leader to start, an instance found running is
	// left running. The status read now tells which, and which resources
	// were up already.
	restore := !actioncontext.IsRollbackDisabled(ctx2) && !actioncontext.IsLeader(ctx2)
	var before instance.Status
	if restore {
		before, err = t.lockedStatusBeforeProvision(ctx)
		if err != nil {
			return err
		}
	}
	if err := t.lockedProvision(ctx2); err != nil {
		return err
	}
	if !restore {
		return nil
	}
	if runningBeforeProvision(before) {
		t.log.Infof("leave the resources started: the instance was running before the provision")
		return nil
	}
	return t.lockedProvisionStop(ctx, before)
}

// runningBeforeProvision tells whether the instance was running before a
// provision: the resources it had provisioned already, of those counting in
// its availability, were up.
//
// A resource being added to a running instance is not provisioned yet, and
// down, which makes the instance availability warn: it does not say the
// instance is not running. A resource provisioned and down does, and an
// instance standing by with a disk up is not taken for a running one.
func runningBeforeProvision(before instance.Status) bool {
	avail := status.NotApplicable
	for rid, rs := range before.Resources {
		if rs.IsOptional || rs.IsDisabled || rs.IsEncap {
			continue
		}
		if rs.IsProvisioned.State != provisioned.True {
			continue
		}
		if id, err := resourceid.Parse(rid); err == nil {
			switch id.DriverGroup() {
			case driver.GroupSync, driver.GroupTask:
				continue
			}
		}
		avail.Add(rs.Status)
	}
	return avail == status.Up
}

// lockedStatusBeforeProvision evaluates the status of the instance, all its
// resources, before a provision changes it.
func (t *actor) lockedStatusBeforeProvision(ctx context.Context) (instance.Status, error) {
	ctx = actioncontext.WithProps(ctx, actioncontext.Status)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	return t.lockedStatusEval(ctx)
}

// lockedProvisionStop stops what a provision started on an instance it found
// down, and leaves alone the resources that were up before it. It is a step
// of the provision, so it flags nothing stopped on purpose: the user asked
// for a provision, not for a stop.
func (t *actor) lockedProvisionStop(ctx context.Context, before instance.Status) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.ProvisionStop)
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		if rs, ok := before.Resources[r.RID()]; ok && rs.Status.Is(status.Up, status.StandbyUp) {
			t.log.Attr("rid", r.RID()).Tracef("%s: leave started, it was %s before the provision", r.RID(), rs.Status)
			return nil
		}
		return resource.Stop(ctx, r)
	})
}

// lockedProvision provisions the resources, and restores the heads it moves
// from unprovisioned to provisioned from the snapshot restore_from names, as
// each is provisioned and started: the heads before the resources using
// their data start.
func (t *actor) lockedProvision(ctx context.Context) error {
	src, err := t.provisionRestoreSource(ctx)
	if err != nil {
		return err
	}
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		rid := r.RID()
		t.log.Attr("rid", rid).Tracef("%s: provision resource", rid)
		leader := actioncontext.IsLeader(ctx)
		wasProvisioned := true
		if src != nil && src.run.Has(rid) {
			// A state not known is taken for provisioned: data is
			// only written into a head known to be new.
			if st, err := resource.Provisioned(ctx, r); err == nil {
				wasProvisioned = st == provisioned.True
			}
		}
		if err := resource.Provision(ctx, r, leader); err != nil {
			return err
		}
		if src != nil && !wasProvisioned {
			return t.restoreProvisionedHead(ctx, src, r)
		}
		return nil
	})
}

// ErrProvisionConflict is the refusal of a provision finding on the node a
// system resource holding what it would take.
var ErrProvisionConflict = errors.New("the provision would take what something on the node holds")

// lockedProvisionPreflight checks that nothing on the node holds what the
// resources not provisioned yet would take: a filesystem mounted on the
// mount point, a volume group of the name, a device in use. A provision would
// otherwise adopt it, or format it.
//
// A resource a provision made on the node already is not checked: what it
// holds is its own. Its provisioned state does not tell: a resource is found
// provisioned when what it names exists, whoever made it.
func (t *actor) lockedProvisionPreflight(ctx context.Context) error {
	conflicts := make([]string, 0)
	for _, r := range t.Resources() {
		if r.IsDisabled() || r.IsActionDisabled() || r.IsProvisionDisabled() {
			// What a sysadmin provisions is the sysadmin's to check.
			continue
		}
		p, ok := r.(resource.ProvisionPreflighter)
		if !ok {
			continue
		}
		if resource.ProvisionRecorded(r) {
			continue
		}
		rctx := resource.WithProvisionCreates(ctx, setsProvisioningKeyword(t.config, r))
		for _, msg := range p.ProvisionConflicts(rctx) {
			conflicts = append(conflicts, fmt.Sprintf("%s: %s", r.RID(), msg))
		}
	}
	if len(conflicts) == 0 {
		t.log.Infof("provision preflight: nothing on the node holds what the provision would take")
		return nil
	}
	for _, msg := range conflicts {
		t.log.Errorf("provision preflight: %s", msg)
	}
	return fmt.Errorf("%w: %s: provision with --adopt to take them over", ErrProvisionConflict, strings.Join(conflicts, "; "))
}

// setsProvisioningKeyword says whether the configuration sets, for the
// resource, a keyword its driver uses only to provision it, as a template or a
// virt-install command: its provision then creates what the resource names.
func setsProvisioningKeyword(cfg *xconfig.T, r resource.Driver) bool {
	options := make(map[string]bool)
	for _, kw := range r.Manifest().Keywords() {
		if kw.Provisioning {
			options[kw.Option] = true
		}
	}
	for _, name := range cfg.Keys(r.RID()) {
		option, _, _ := strings.Cut(name, "@")
		if options[option] {
			return true
		}
	}
	return false
}
