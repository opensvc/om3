package netwarden

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	drvnetwarden "github.com/opensvc/om3/v3/drivers/shared/netwarden"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// rules is what the configuration of an object says of the filter of one of
// its resources, as its driver reads it.
type rules struct {
	// net is the rules, evaluated for this node.
	net drvnetwarden.T

	// target is what the filter is applied to.
	target drvnetwarden.Target

	// issue returns what keeps a filter from being set under what the
	// resource already runs, having started with no rule.
	issue func() error
}

// issueTimeout bounds what a driver asks its engine to say whether what it
// runs can be filtered.
const issueTimeout = 10 * time.Second

var (
	// configured returns what the configuration of an object of the node
	// says of the filters of its resources, by resource, the ones with
	// no rule included, and is replaced by the tests.
	//
	// The object is read as an action reads it, and each resource is
	// configured by its driver: the rules are the keyword as a start
	// would evaluate it, with its references, its scoping and what
	// it inherits of the DEFAULT section, and the process group, the
	// directory of the record and the nameservers are the ones a start
	// would use.
	configured = func(p naming.Path) (map[string]rules, error) {
		o, err := object.NewActor(p, object.WithVolatile(true))
		if errors.Is(err, object.ErrWrongType) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		l := make(map[string]rules)
		for _, r := range o.Resources() {
			d, ok := r.(drvnetwarden.Driver)
			if !ok {
				continue
			}
			net, target := d.NetRules()
			l[r.RID()] = rules{
				net:    net,
				target: target,
				issue: func() error {
					ctx, cancel := context.WithTimeout(context.Background(), issueTimeout)
					defer cancel()
					return d.NetLiveIssue(ctx)
				},
			}
		}
		return l, nil
	}

	// namesRules says the configuration file of an object of the node
	// holds the keyword, in any section and whatever it is scoped to,
	// and is replaced by the tests: a configuration not holding it
	// gives no resource a rule, and is not worth reading as an
	// action does, for every object of the node as the daemon starts.
	namesRules = func(p naming.Path) bool {
		b, err := os.ReadFile(p.ConfigFile())
		if err != nil {
			return false
		}
		return bytes.Contains(b, []byte(drvnetwarden.Option))
	}
)

// converge applies the rules of the configuration of an object of the node
// to the resources it runs, where they are not the ones their filter was
// resolved from: the configuration changed under them, or did while no
// daemon was there to hear of it.
//
// It is the exception to the rule that a configuration change waits for the
// next action: a filter is a security setting, and one left as it was until
// a restart is one nobody restarts for.
//
// The access to the rules was judged when the configuration was written:
// nothing is asked again here.
func (t *Manager) converge(p naming.Path) {
	if !t.has(p) && !namesRules(p) {
		t.log.Debugf("%s: its configuration holds no %s, and none of its resources has a filter: no rule to apply", p, drvnetwarden.Option)
		return
	}
	wanted, err := configured(p)
	if err != nil {
		t.log.Warnf("%s: read the rules of the configuration: %s", p, err)
		return
	}
	recorded := make(map[string]bool)
	for _, e := range t.sorted() {
		if e.path != p {
			continue
		}
		recorded[e.rid] = true
		w, ok := wanted[e.rid]
		switch {
		case !ok:
			// A resource the configuration no longer has, or that
			// takes no rule: its filter is the one of its start
			// until its stop lifts it.
			t.log.Debugf("%s %s: the configuration has no such resource taking rules: its filter is left to its stop", p, e.rid)
		case !w.net.Enabled():
			t.lift(e)
		case !e.rec.HasRules(w.net.Rules):
			// The rules are compared, in their order, not the text
			// of the keyword: the same rules laid out another way
			// set no filter again.
			t.replace(e, w)
		default:
			t.log.Debugf("%s %s: the rules of the configuration are the ones of its filter: unchanged", p, e.rid)
		}
	}
	rids := make([]string, 0, len(wanted))
	for rid, w := range wanted {
		if w.net.Enabled() && !recorded[rid] {
			rids = append(rids, rid)
		}
	}
	slices.Sort(rids)
	for _, rid := range rids {
		t.adopt(p, rid, wanted[rid])
	}
}

// has says a resource of the object has a filter recorded.
func (t *Manager) has(p naming.Path) bool {
	for _, e := range t.entries {
		if e.path == p {
			return true
		}
	}
	return false
}

// replace gives a running resource the rules its configuration has now, in
// place of the ones its filter was resolved from, and sets the filter they
// resolve to.
//
// Rules the grammar refuses, or no filter is made of, change nothing: the
// filter in force is kept, and the status of the resource says so until the
// rules are fixed.
func (t *Manager) replace(e *entry, w rules) {
	rec, err := e.rec.WithRules(w.net.Rules)
	if err != nil {
		t.log.Warnf("%s %s: the rules changed are refused, and the filter is kept, %s on %s: %s", e.path, e.rid, e.rec.Filter.Describe(), where(e.rec), err)
		return
	}
	next := t.index(e.file, e.path, rec)
	next.reapply = e.reapply
	next.rules = true
	t.entries[e.file] = next
	if t.refresh(next) {
		return
	}
	if _, ok := t.entries[e.file]; ok {
		// Not set: the resource keeps the rules of its filter, to be
		// replaced by the next change.
		t.entries[e.file] = e
	}
}

// lift lifts the filter of a running resource whose configuration has no
// rule any more, and forgets it, as its stop would.
func (t *Manager) lift(e *entry) {
	s := backendOf(e.rec)
	if s.HasFilter() {
		if err := s.Set(netrule.Filter{}); err != nil {
			t.log.Errorf("%s %s: lift the filter of %s: %s", e.path, e.rid, where(e.rec), err)
			return
		}
	}
	if err := os.Remove(e.file); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.log.Warnf("%s %s: %s", e.path, e.rid, err)
	}
	delete(t.entries, e.file)
	t.log.Infof("%s %s: rules removed: no filter on %s, was %s", e.path, e.rid, where(e.rec), e.rec.Filter.Describe())
	t.outdate(e)
}

// adopt gives a filter to a resource running with none, its configuration
// having been given rules since it started.
//
// What it runs is filtered if it is in a unit of the slice of its process
// group: the scope an engine made of a container, the one a lxc container
// is started in, or the service of a command started with
// systemd_properties. A command started in no unit has its processes in the
// group itself, and a lxc container started by an agent making no scope has
// them in groups of its own under it, where systemd filters nothing: it is
// warned of, here and by the status of the resource, and filtered from its
// next start.
func (t *Manager) adopt(p naming.Path, rid string, w rules) {
	rec := w.net.Record(w.target)
	if (rec.PG == "" && len(rec.Devs) == 0) || w.target.VarDir == nil {
		t.log.Debugf("%s %s: rules, and no process group nor interface to filter", p, rid)
		return
	}
	s := backendOf(rec)
	if !s.HasMembers() {
		if s.HasProcs() {
			// What runs is in no unit: a command, or a container
			// its manager placed by itself. Its driver says why
			// when its next start would be refused the rules too.
			if w.issue != nil {
				if err := w.issue(); err != nil {
					t.log.Warnf("%s %s: the rules are not applied to what the resource runs, unfiltered until it is restarted: %s", p, rid, err)
					return
				}
			}
			t.log.Warnf("%s %s: %s", p, rid, drvnetwarden.NoUnitWarning)
		} else {
			t.log.Debugf("%s %s: rules and no filter recorded, nothing running in %s: its start sets its filter", p, rid, where(rec))
		}
		return
	}
	t.log.Debugf("%s %s: rules and no filter recorded, something running in %s: given rules since it started", p, rid, where(rec))
	if _, err := w.net.Parse(w.target.Backend); err != nil {
		// Read from the configuration, for the error to name the line
		// of the keyword: the record counts its rules.
		t.log.Warnf("%s %s: the rules are refused, and the resource runs unfiltered: %s", p, rid, err)
		return
	}
	if w.issue != nil {
		if err := w.issue(); err != nil {
			t.log.Warnf("%s %s: the rules are not applied to what the resource runs, unfiltered until it is restarted: %s", p, rid, err)
			return
		}
	}
	e := t.index(filepath.Join(w.target.VarDir(), recordName), p, rec)
	e.rules, e.unrecorded = true, true
	t.entries[e.file] = e
	if !t.refresh(e) {
		delete(t.entries, e.file)
	}
}

// outdate tells the status of the instance of a resource is outdated: it
// reads the record, which the daemon wrote or removed.
func (t *Manager) outdate(e *entry) {
	if t.publisher == nil {
		return
	}
	t.publisher.Pub(&msgbus.InstanceStateFileUpdated{Path: e.path, Node: t.localhost, File: filepath.Join(e.rid, recordName)},
		pubsub.Label{"node", t.localhost},
		pubsub.Label{"namespace", e.path.Namespace},
		pubsub.Label{"path", e.path.String()},
	)
}
