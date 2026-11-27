package rescontainerocibase

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/drivers/shared/netwarden"
	"github.com/opensvc/om3/v3/util/pg"
)

// managerArgs is an argser of an engine printing the cgroup manager it is
// given, run by the credential it is given.
type managerArgs struct {
	ExecutorArgser
	cred    *Credential
	manager string
}

func (a managerArgs) Credential() (*Credential, error) {
	return a.cred, nil
}

func (a managerArgs) CgroupManagerArgs() []string {
	return []string{"-c", "echo " + a.manager}
}

// A container is filtered in the scope its engine makes of it with the
// systemd cgroup manager, as a unit of the systemd of the system: an engine
// placing it with cgroupfs, or running it rootless, is said before the
// container is started.
func TestNetFilterIssue(t *testing.T) {
	ctx := context.Background()

	e := NewExecutor("/bin/sh", managerArgs{manager: "systemd"}, testLogger{})
	assert.NoError(t, e.NetFilterIssue(ctx))

	e = NewExecutor("/bin/sh", managerArgs{manager: "cgroupfs"}, testLogger{})
	assert.EqualError(t, e.NetFilterIssue(ctx),
		"/bin/sh places its containers with the cgroupfs cgroup manager, in no systemd unit of the slice the filter is set on: it takes the systemd one")

	e = NewExecutor("/bin/sh", managerArgs{manager: "systemd", cred: nobody}, testLogger{})
	assert.EqualError(t, e.NetFilterIssue(ctx),
		"the container is run rootless, by the uid 65534, as a unit of the systemd instance of that user, which may not filter the network")

	e = NewExecutor("/bin/false", managerArgs{manager: "systemd"}, testLogger{})
	assert.ErrorContains(t, e.NetFilterIssue(ctx), "/bin/false -c echo systemd: ", "an engine that does not answer is not one known to filter")

	e = NewExecutor("/bin/sh", credArgs{}, testLogger{})
	assert.NoError(t, e.NetFilterIssue(ctx), "an engine not saying its manager is verified once the container runs")
}

// The start of a container whose filter could not be enforced is refused
// before the engine is asked anything, and nothing is set on the slice.
func TestNetApplyRefusesAContainerItCanNotFilter(t *testing.T) {
	ctx := context.Background()
	newBT := func(manager string) *BT {
		bt := &BT{}
		require.NoError(t, bt.SetRID("container#1"))
		bt.Net = netwarden.T{Rules: "allow node n2"}
		_ = bt.WithExecuter(NewExecutor("/bin/sh", managerArgs{manager: manager}, testLogger{}))
		return bt
	}

	bt := newBT("systemd")
	filtered, err := bt.netApply(ctx)
	assert.False(t, filtered)
	assert.ErrorIs(t, err, pg.ErrNoSlice, "no process group")

	bt.SetPG(&pg.Config{ID: "/opensvc/svc.web/container.1"})
	_, err = bt.netApply(ctx)
	assert.ErrorIs(t, err, pg.ErrNoSlice, "a process group that is no slice")

	bt = newBT("systemd")
	bt.Net.Rules = "allow node n2\ndeny any port 22 proto tcp"
	bt.SetPG(&pg.Config{ID: "/opensvc.slice/opensvc-svc.web.slice/opensvc-svc.web-container.1.slice"})
	_, err = bt.netApply(ctx)
	assert.ErrorContains(t, err, "netwarden: net_rules line 2: \"deny any port 22 proto tcp\": port 22: the cgroup filter")

	// No rule: nothing is asked of the engine, and nothing refused.
	bt = newBT("cgroupfs")
	bt.Net = netwarden.T{}
	filtered, err = bt.netApply(ctx)
	assert.NoError(t, err)
	assert.False(t, filtered)
}

// A restart policy is found in the run args in the two forms the engines
// take, with its value, and in no other option.
func TestRestartOption(t *testing.T) {
	for want, l := range map[string][]string{
		"--restart always":         {"-p", "80:80", "--restart", "always"},
		"--restart=always":         {"--restart=always", "-p", "80:80"},
		"--restart=on-failure:3":   {"--restart=on-failure:3"},
		"--restart unless-stopped": {"--restart", "unless-stopped", "--init"},
		"--restart=no":             {"--restart=no"},
		"--restart":                {"--init", "--restart"},
		"--restart=":               {"--restart="},
		"--restart on-failure:3":   {"--restart", "on-failure:3"},
	} {
		assert.Equal(t, want, restartOption(l))
	}
	for _, l := range [][]string{
		nil,
		{"-p", "80:80", "--init"},
		{"--restart-sec", "3"},
		{"--label", "restart=always"},
		{"-e", "X=--restart=always"},
	} {
		assert.Empty(t, restartOption(l), "%v", l)
	}
}

// A container with a network filter and a restart policy in its run args is
// a configuration error, and its start is refused before anything is set:
// the engine would restart it with no filter.
func TestNetRefusesARestartPolicy(t *testing.T) {
	ctx := context.Background()
	newBT := func(net netwarden.T, runArgs ...string) *BT {
		bt := &BT{RunArgs: runArgs, Net: net, Detach: true}
		require.NoError(t, bt.SetRID("container#1"))
		bt.SetPG(&pg.Config{ID: "/opensvc.slice/opensvc-svc.web.slice/opensvc-svc.web-container.1.slice"})
		_ = bt.WithExecuter(NewExecutor("/bin/sh", managerArgs{manager: "systemd"}, testLogger{}))
		return bt
	}
	const refusal = "netwarden: run_args holds %s, which has the engine restart the container by itself, at boot or when it exits, before om sets its network filter: with net_rules the restart policy is left out, and om restarts the container"

	for _, net := range []netwarden.T{{Rules: "allow node n2"}, {Rules: "deny node n2\nallow any"}} {
		for option, runArgs := range map[string][]string{
			"--restart always":         {"--restart", "always"},
			"--restart=always":         {"-p", "80:80", "--restart=always"},
			"--restart unless-stopped": {"--restart", "unless-stopped"},
			"--restart":                {"--restart"},
		} {
			bt := newBT(net, runArgs...)
			want := fmt.Sprintf(refusal, option)
			assert.EqualError(t, bt.netRestartIssue(), want)
			assert.EqualError(t, bt.Configure(), want)
			filtered, err := bt.netApply(ctx)
			assert.False(t, filtered)
			assert.EqualError(t, err, want, "refused at the start too, as the container of a task is")
		}
	}

	// No rule: the restart policy is the operator's, as before.
	bt := newBT(netwarden.T{}, "--restart=always")
	assert.NoError(t, bt.netRestartIssue())
	assert.NoError(t, bt.Configure())

	// Rules and no restart policy.
	bt = newBT(netwarden.T{Rules: "allow node n2"}, "-p", "80:80")
	assert.NoError(t, bt.netRestartIssue())
	assert.NoError(t, bt.Configure())
}

// The filter of a container is the one of the slice om names its cgroup
// parent: an option of the run args placing the container elsewhere, read
// after the one of om, is refused with rules, at the configuration and at
// the start, a container run in the foreground having no filter verified.
func TestNetRefusesAPlacementOption(t *testing.T) {
	ctx := context.Background()
	newBT := func(net netwarden.T, detach bool, runArgs ...string) *BT {
		bt := &BT{RunArgs: runArgs, Net: net, Detach: detach}
		require.NoError(t, bt.SetRID("container#1"))
		bt.SetPG(&pg.Config{ID: "/opensvc.slice/opensvc-svc.web.slice/opensvc-svc.web-container.1.slice"})
		_ = bt.WithExecuter(NewExecutor("/bin/sh", managerArgs{manager: "systemd"}, testLogger{}))
		return bt
	}
	const refusal = "netwarden: run_args holds %s, which has the engine place the container in the cgroups by itself, out of the slice om sets its network filter on: with net_rules the placement is left out, and om places the container in the slice of its process group"

	cases := map[string][]string{
		"--cgroup-parent /elsewhere.slice": {"--cgroup-parent", "/elsewhere.slice"},
		"--cgroup-parent=/elsewhere.slice": {"-p", "80:80", "--cgroup-parent=/elsewhere.slice"},
		"--cgroup-parent":                  {"--cgroup-parent"},
		"--cgroups disabled":               {"--cgroups", "disabled"},
		"--cgroups=split":                  {"--cgroups=split", "-p", "80:80"},
		"--cgroups=no-conmon":              {"--cgroups=no-conmon"},
		"--cgroups=enabled":                {"--cgroups=enabled"},
	}
	for _, net := range []netwarden.T{{Rules: "allow node n2"}, {Rules: "deny node n2\nallow any"}} {
		for _, detach := range []bool{true, false} {
			for option, runArgs := range cases {
				bt := newBT(net, detach, runArgs...)
				want := fmt.Sprintf(refusal, option)
				assert.EqualError(t, bt.netPlacementIssue(), want)
				assert.EqualError(t, bt.Configure(), want)
				filtered, err := bt.netApply(ctx)
				assert.False(t, filtered)
				assert.EqualError(t, err, want, "refused at the start too, as the container of a task is")
			}
		}
	}

	// No rule: the placement is the operator's, as before.
	for _, runArgs := range cases {
		bt := newBT(netwarden.T{}, true, runArgs...)
		assert.NoError(t, bt.netPlacementIssue())
		assert.NoError(t, bt.Configure())
	}

	// Rules and no placement option, the namespace of the cgroups being no
	// placement.
	bt := newBT(netwarden.T{Rules: "allow node n2"}, true, "-p", "80:80", "--cgroupns=host")
	assert.NoError(t, bt.netPlacementIssue())
	assert.NoError(t, bt.Configure())
}
