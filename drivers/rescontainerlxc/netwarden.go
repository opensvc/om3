package rescontainerlxc

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/drivers/rescontainer"
	"github.com/opensvc/om3/v3/drivers/shared/netwarden"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/cgroupbpf"
	"github.com/opensvc/om3/v3/util/pg"
	"github.com/opensvc/om3/v3/util/sdunit"
	"github.com/opensvc/om3/v3/util/systemd"
)

// scopeEndTimeout is how long the scope of a container that stopped is
// waited for to end: its last process, the monitor, exits after lxc-stop
// returns.
const scopeEndTimeout = 10 * time.Second

var (
	// netKeywords is net_rules, in the group the networks
	// of a guest are in, as the ones of a kvm container: the isolation
	// group of the docker and podman containers holds the namespaces and
	// the privileges their engine is asked for, which lxc reads from its
	// own configuration file. Their filter is the one of the cgroups,
	// which knows no port.
	netKeywords = netwarden.Keywords(rescontainer.GroupNetwork.ID)

	// hasSystemd says the node runs systemd, replaced by the tests.
	hasSystemd = systemd.HasSystemd

	// isUnified says the node holds the unified cgroup hierarchy alone,
	// replaced by the tests.
	isUnified = pg.IsUnified

	// hasCgroupDir says lxc takes lxc.cgroup.dir, replaced by the tests.
	hasCgroupDir = func() bool {
		return capabilities.Has(drvID.Cap() + ".cgroup_dir")
	}

	// cgroupOf returns the cgroup directory of a process, replaced by
	// the tests.
	cgroupOf = cgroupbpf.Dir

	// scopeResetFailed and scopeWaitInactive ask systemd about the scope,
	// replaced by the tests.
	scopeResetFailed  = sdunit.ResetFailed
	scopeWaitInactive = sdunit.WaitInactive
)

// scopeSlice returns the systemd slice the process group of the resource
// is, empty when it is none, or is delegated to a user.
func (t *T) scopeSlice() string {
	c := t.GetPG()
	if c == nil || c.ID == "" {
		return ""
	}
	return c.Slice()
}

// scopeName returns the name of the transient scope the container is
// started in, a unit of the slice of its process group, and an empty string
// when it is started in none.
//
// lxc makes the groups of a container by itself, straight under the group it
// is given, in no unit: systemd, which filters the units of a slice and
// nothing else in it, would filter nothing of the container. Started in a
// scope of the slice, and told to make its groups under the one of the
// scope, the container is a unit of the slice.
//
// Every container is started so where it can be, with rules or not: the
// groups of the containers of a node are laid out one way, and rules given
// to a running container apply at once. It can be on a node running systemd
// on the unified hierarchy alone, the one a filter is attached on, for a
// process group that is a slice, with a lxc taking lxc.cgroup.dir. Elsewhere
// the container is started as it always was, where no filter could be set.
func (t *T) scopeName() string {
	slice := t.scopeSlice()
	if slice == "" || !hasSystemd() || !isUnified() || !hasCgroupDir() {
		return ""
	}
	return strings.TrimSuffix(slice, ".slice") + "-lxc.scope"
}

// lxcCgroupDir returns the group lxc is told to make the groups of the
// container under, relative to the root of the hierarchy: the one of the
// scope when the container is started in one, else the process group of the
// resource.
//
// The group of the scope is spelled out, rather than left to
// lxc.cgroup.relative, which a configuration file may set either way.
func (t *T) lxcCgroupDir() string {
	dir := t.cgroupDir()
	if scope := t.scopeName(); scope != "" {
		return dir + "/" + scope
	}
	return dir
}

// startCommand returns the command starting the container, from the
// arguments of lxc-start: lxc-start itself, or systemd-run making the scope
// lxc-start runs in.
//
// The scope is delegated, lxc making groups of its own under it. It is
// ordered before the om daemon, as the units of the apps are: at a reboot,
// its stop job comes after the one of the daemon, which shuts the container
// down first, in the order of the resources, where systemd would signal its
// processes at once.
func (t *T) startCommand(lxcArgs []string) (string, []string) {
	scope := t.scopeName()
	if scope == "" {
		return "lxc-start", lxcArgs
	}
	args := []string{
		"--quiet",
		"--scope",
		"--slice=" + t.scopeSlice(),
		"--unit=" + scope,
		"--property=Delegate=yes",
		"--property=Before=" + sdunit.ServerUnit,
		"--",
		"lxc-start",
	}
	return "systemd-run", append(args, lxcArgs...)
}

// scopePrepare makes the name of the scope free for a start: a scope that
// failed is forgotten, and the one of a container that just stopped is
// waited for to end.
func (t *T) scopePrepare(ctx context.Context) error {
	scope := t.scopeName()
	if scope == "" {
		return nil
	}
	if _, err := scopeWaitInactive(ctx, scope, scopeEndTimeout); err != nil {
		return err
	}
	return scopeResetFailed(ctx, scope)
}

// scopeEnd waits for the scope of the container that stopped to end, for
// the filter of its slice to be lifted: it is kept while the slice holds a
// unit. A scope still there is said, and its filter is kept.
func (t *T) scopeEnd(ctx context.Context) {
	scope := t.scopeName()
	if scope == "" {
		return
	}
	if ok, err := scopeWaitInactive(ctx, scope, scopeEndTimeout); err != nil {
		t.Log().Warnf("%s", err)
	} else if !ok {
		t.Log().Warnf("the scope %s still runs processes %s after the container stopped", scope, scopeEndTimeout)
	}
}

// netTarget is what the network filter of the container is applied to: the
// slice of its process group, the scope of the container is a unit of.
//
// It says no nameservers of its own: om writes no resolver for a lxc
// container, so the ones the dns rule names are the ones of the resolver of
// the node.
func (t *T) netTarget() netwarden.Target {
	return netwarden.Target{
		Path:   t.Path,
		PG:     t.GetPG(),
		VarDir: t.VarDir,
		Log:    t.Log(),
	}
}

// NetRules implements netwarden.Driver.
func (t *T) NetRules() (netwarden.T, netwarden.Target) {
	return t.Net, t.netTarget()
}

// NetLiveIssue implements netwarden.Driver: what refuses a start its
// filter, and a container running out of the scope a filter reaches, as one
// started by an agent that made none.
func (t *T) NetLiveIssue(ctx context.Context) error {
	if err := t.netIssue(); err != nil {
		return err
	}
	pid, err := t.getPID(ctx)
	if err != nil {
		return fmt.Errorf("netwarden: the process of the container %s: %w", t.Name, err)
	}
	if pid == 0 {
		return nil
	}
	return t.netPlacementIssue(pid)
}

// netIssue returns what keeps the network filter of the container from
// being enforced, nil when nothing does, or when it has no rule.
func (t *T) netIssue() error {
	if !t.Net.Enabled() {
		return nil
	}
	if err := t.Net.Check(t.netTarget()); err != nil {
		return err
	}
	if t.scopeName() != "" {
		return nil
	}
	reason := "the node does not hold the unified cgroup hierarchy alone"
	if !hasCgroupDir() {
		reason = "this lxc takes no lxc.cgroup.dir, as of the last scan of the capabilities of the node"
	}
	return fmt.Errorf("netwarden: %s can not be applied: the container is started in no systemd scope of the slice the filter is set on: %s", netwarden.Option, reason)
}

// netPlacementIssue says the process pid of the container runs out of the
// scope of the slice of the process group, nil when it runs in it.
func (t *T) netPlacementIssue(pid int) error {
	dir, err := cgroupOf(pid)
	if err != nil {
		return fmt.Errorf("netwarden: the cgroup of the process %d: %w", pid, err)
	}
	scope := t.scopeName()
	if scope != "" && strings.HasPrefix(dir, filepath.Join(t.GetPG().Dir(), scope)+string(filepath.Separator)) {
		return nil
	}
	return fmt.Errorf("netwarden: the container %s runs in the cgroup %s, in no systemd scope of the slice the filter is set on, as started by an agent making none: the rules apply at its next start", t.Name, dir)
}

// netApply sets the network filter of the container on the slice of its
// process group, before the container is started in a scope of it, and says
// whether a filter is in force. A container with no rule has the filter an
// earlier configuration left lifted.
//
// A filter that could not be enforced is refused before anything is set, so
// the container is not started unfiltered.
func (t *T) netApply() (bool, error) {
	target := t.netTarget()
	if !t.Net.Enabled() {
		_, err := t.Net.Apply(target)
		return false, err
	}
	if err := t.netIssue(); err != nil {
		return false, err
	}
	return t.Net.Apply(target)
}

// netVerify says the kernel filters the container started: its first
// process is in the scope, with the filter attached.
func (t *T) netVerify(ctx context.Context) error {
	pid, err := t.getPID(ctx)
	if err != nil {
		return fmt.Errorf("netwarden: the process of the container to verify the filter of: %w", err)
	}
	if pid == 0 {
		return fmt.Errorf("netwarden: the container has no process to verify the filter of")
	}
	return netwarden.Verify(t.netTarget(), pid)
}

// netAdopt gives the container found running at a start the filter of its
// rules, as the daemon does, and warns of a container no filter reaches,
// left running.
func (t *T) netAdopt(ctx context.Context) {
	if !t.Net.Enabled() {
		return
	}
	err := t.NetLiveIssue(ctx)
	if err == nil {
		var filtered bool
		if filtered, err = t.netApply(); err == nil && filtered {
			err = t.netVerify(ctx)
		}
	}
	if err != nil {
		t.Log().Warnf("%s: the container runs unfiltered until it is restarted", err)
	}
}

// netClear lifts the network filter of the container once it is stopped. A
// failure is warned of: the container stopped.
func (t *T) netClear() {
	if err := t.Net.Clear(t.netTarget()); err != nil {
		t.Log().Warnf("%s", err)
	}
}

// netStatus adds what the status of the running container says of its
// network filter to its status log.
func (t *T) netStatus() {
	infos, warnings := t.Net.Status(t.netTarget())
	for _, s := range infos {
		t.StatusLog().Info("%s", s)
	}
	for _, s := range warnings {
		t.StatusLog().Warn("%s", s)
	}
}
