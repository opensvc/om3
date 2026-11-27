package rescontainerocibase

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"github.com/opensvc/om3/v3/drivers/shared/netwarden"
)

type (
	// CgroupManagerArgser is an optional interface an ExecutorArgser
	// implements for an engine saying which cgroup manager it places its
	// containers with: the arguments of the command printing it, systemd
	// or cgroupfs.
	CgroupManagerArgser interface {
		CgroupManagerArgs() []string
	}

	// netFilterIssuer is implemented by an executer knowing what keeps
	// its engine from running a container filtered.
	netFilterIssuer interface {
		NetFilterIssue(ctx context.Context) error
	}
)

// netKeywords is net_rules, beside the namespaces and the
// privileges of the container.
var netKeywords = netwarden.Keywords(GroupIsolation.ID)

// NetFilterIssue returns what keeps the engine from running a container
// whose network systemd filters, nil when nothing does.
//
// systemd filters the units of the slice of the process group, which an
// engine makes of its container with the systemd cgroup manager alone: with
// cgroupfs, the container is in a cgroup systemd knows nothing of. And a
// rootless container is a unit of the systemd instance of its user, which
// may not attach a filter.
func (e *Executor) NetFilterIssue(ctx context.Context) error {
	cred, err := e.credential()
	if err != nil {
		return err
	}
	if cred != nil {
		return fmt.Errorf("the container is run rootless, by the uid %d, as a unit of the systemd instance of that user, which may not filter the network", cred.UID)
	}
	i, ok := e.args.(CgroupManagerArgser)
	if !ok {
		return nil
	}
	a := i.CgroupManagerArgs()
	cmd, err := e.command(ctx, a...)
	if err != nil {
		return err
	}
	b, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("%s %s: %w", e.bin, strings.Join(a, " "), err)
	}
	if manager := strings.TrimSpace(string(b)); manager != "systemd" {
		return fmt.Errorf("%s places its containers with the %s cgroup manager, in no systemd unit of the slice the filter is set on: it takes the systemd one", e.bin, manager)
	}
	return nil
}

// restartOption returns the option of the run args setting the restart
// policy of the container, as "--restart x" or "--restart=x", the forms both
// docker and podman take, and an empty string when none does.
func restartOption(l []string) string {
	for i, s := range l {
		switch {
		case s == "--restart":
			if i+1 < len(l) {
				return s + " " + l[i+1]
			}
			return s
		case strings.HasPrefix(s, "--restart="):
			return s
		}
	}
	return ""
}

// netRestartIssue refuses a restart policy in the run args of a container
// with a network filter, nil when there is none.
//
// om passes no restart policy, so the engine restarts no container by
// itself, and the daemon starts them, the filter set on the slice first. A
// policy set in run_args has the engine restart the container at boot, or
// when it exits, with nobody setting the filter the slice lost: it would run
// unfiltered. Whatever the policy, "no" included: what is refused is the
// option, not judged by its value.
func (t *BT) netRestartIssue() error {
	if !t.Net.Enabled() {
		return nil
	}
	option := restartOption(t.RunArgs)
	if option == "" {
		return nil
	}
	return fmt.Errorf("netwarden: run_args holds %s, which has the engine restart the container by itself, at boot or when it exits, before om sets its network filter: with %s the restart policy is left out, and om restarts the container", option, netwarden.Option)
}

// placementOption returns the option of the run args placing the container
// in the cgroups, "--cgroup-parent" of docker and podman or "--cgroups" of
// podman, as "--option x" or "--option=x", and an empty string when none
// does.
func placementOption(l []string) string {
	for i, s := range l {
		for _, option := range []string{"--cgroup-parent", "--cgroups"} {
			switch {
			case s == option:
				if i+1 < len(l) {
					return s + " " + l[i+1]
				}
				return s
			case strings.HasPrefix(s, option+"="):
				return s
			}
		}
	}
	return ""
}

// netPlacementIssue refuses an option placing the container in the cgroups
// in the run args of a container with a network filter, nil when there is
// none.
//
// The filter is the one of the slice of the process group, which om names
// the cgroup parent of the container, the engine making a scope of that
// slice of it. The run args are the last options of the command line, so a
// "--cgroup-parent" there wins over the one of om, and a "--cgroups" of
// podman has the container in no scope, or in the cgroup of the command: the
// container would run out of the slice, unfiltered. The start of a detached
// container verifies its filter, where nothing verifies the one of a
// container run in the foreground, as the one of a task. Whatever the value:
// what is refused is the option, not judged by its value.
func (t *BT) netPlacementIssue() error {
	if !t.Net.Enabled() {
		return nil
	}
	option := placementOption(t.RunArgs)
	if option == "" {
		return nil
	}
	return fmt.Errorf("netwarden: run_args holds %s, which has the engine place the container in the cgroups by itself, out of the slice om sets its network filter on: with %s the placement is left out, and om places the container in the slice of its process group", option, netwarden.Option)
}

// netTarget is what the network filter of the container is applied to: the
// slice of its process group, the cgroup parent of the container.
func (t *BT) netTarget() netwarden.Target {
	return netwarden.Target{
		Path:   t.Path,
		PG:     t.GetPG(),
		VarDir: t.VarDir,
		Log:    t.Log(),
		DNS:    func() ([]netip.Addr, error) { return t.netNameservers(), nil },
	}
}

// NetRules implements netwarden.Driver.
func (t *BT) NetRules() (netwarden.T, netwarden.Target) {
	return t.Net, t.netTarget()
}

// NetLiveIssue implements netwarden.Driver: what refuses a start its
// filter, which keeps a filter set under the running container from being
// one the engine holds it to.
func (t *BT) NetLiveIssue(ctx context.Context) error {
	return t.netIssue(ctx)
}

// netIssue returns what keeps the network filter of the container from
// being enforced, nil when nothing does.
func (t *BT) netIssue(ctx context.Context) error {
	if err := t.netRestartIssue(); err != nil {
		return err
	}
	if err := t.netPlacementIssue(); err != nil {
		return err
	}
	if err := t.Net.Check(t.netTarget()); err != nil {
		return err
	}
	if i, ok := t.executer.(netFilterIssuer); ok {
		if err := i.NetFilterIssue(ctx); err != nil {
			return fmt.Errorf("netwarden: %s can not be applied: %w", netwarden.Option, err)
		}
	}
	return nil
}

// netApply sets the network filter of the container on the slice of its
// process group, before the container is started in it, and says whether a
// filter is in force. A container with no rule has the filter an earlier
// configuration left lifted.
//
// A filter that could not be enforced is refused before anything is set, so
// the container is not started unfiltered.
func (t *BT) netApply(ctx context.Context) (bool, error) {
	target := t.netTarget()
	if !t.Net.Enabled() {
		_, err := t.Net.Apply(target)
		return false, err
	}
	if err := t.netIssue(ctx); err != nil {
		return false, err
	}
	return t.Net.Apply(target)
}

// netVerify says the kernel filters the container the engine started: its
// first process is in a unit of the slice, with the filter attached.
func (t *BT) netVerify(ctx context.Context) error {
	inspect, err := t.executer.InspectRefresh(ctx)
	if err != nil {
		return fmt.Errorf("netwarden: inspect the container to verify its filter: %w", err)
	}
	if inspect == nil || inspect.PID() == 0 {
		return fmt.Errorf("netwarden: the container has no process to verify the filter of")
	}
	return netwarden.Verify(t.netTarget(), inspect.PID())
}

// netClear lifts the network filter of the container once it is stopped. A
// failure is warned of: the container stopped.
func (t *BT) netClear() {
	if err := t.Net.Clear(t.netTarget()); err != nil {
		t.Log().Warnf("%s", err)
	}
}

// netStatus adds what the status of the running container says of its
// network filter to its status log.
func (t *BT) netStatus() {
	infos, warnings := t.Net.Status(t.netTarget())
	for _, s := range infos {
		t.StatusLog().Info("%s", s)
	}
	for _, s := range warnings {
		t.StatusLog().Warn("%s", s)
	}
}
