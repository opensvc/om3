//go:build !windows

package resapp

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/executable"
	"github.com/opensvc/om3/v3/util/pg"
	"github.com/opensvc/om3/v3/util/sdunit"
)

func newUnitT(t *testing.T) *T {
	t.Helper()
	r := &T{StartCmd: "/bin/sleep 10", CheckCmd: "/bin/true"}
	require.NoError(t, r.SetRID("app#main"))
	r.Path = naming.Path{Name: "web", Namespace: "prod", Kind: naming.KindSvc}
	r.SetPG(&pg.Config{ID: "/opensvc.slice/opensvc-ns.prod.slice/opensvc-ns.prod-svc.web.slice/opensvc-ns.prod-svc.web-app.main.slice"})
	r.SystemdProperties = "ProtectSystem=strict PrivateTmp=yes"
	return r
}

// TestUnitName pins that the unit of a resource is named after its process
// group, the slice it is placed in, and after the path of the object and the
// resource when it has none.
func TestUnitName(t *testing.T) {
	r := newUnitT(t)
	assert.True(t, r.UnitMode())
	assert.Equal(t, "opensvc-ns.prod-svc.web-app.main.service", r.UnitName())
	assert.Equal(t, "opensvc-ns.prod-svc.web-app.main.slice", r.unitSlice())

	r.SetPG(&pg.Config{})
	assert.Equal(t, "opensvc-prod.svc.web-app.main.service", r.UnitName())

	r.SystemdProperties = " "
	assert.False(t, r.UnitMode())
}

// TestCmdArgsInUnitModeHasNoProcessGroup pins that om exec does not move a
// command of a resource whose start command runs in a unit to the process
// group: the unit is placed in it, and a command moved out of the cgroup of
// its unit is lost to it.
func TestCmdArgsInUnitModeHasNoProcessGroup(t *testing.T) {
	testhelper.SetExecutable(t, "../..")
	defer executable.Unset()
	defer func(f func() bool) { hasSystemd = f }(hasSystemd)
	hasSystemd = func() bool { return false }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	r := newUnitT(t)
	args, _, err := r.CmdArgs(ctx, r.StartCmd, "start")
	require.NoError(t, err)
	assert.NotContains(t, args, "--pg")
	assert.Equal(t, "exec", args[1])

	r.SystemdProperties = ""
	args, _, err = r.CmdArgs(ctx, r.StartCmd, "start")
	require.NoError(t, err)
	assert.Contains(t, args, "--pg")
}

// TestUnitRunRefusesANodeWithoutSystemd pins that the properties fail the
// start on a node not running systemd, rather than running the command
// unsandboxed.
func TestUnitRunRefusesANodeWithoutSystemd(t *testing.T) {
	defer func(f func() bool) { hasSystemd = f }(hasSystemd)
	hasSystemd = func() bool { return false }
	r := newUnitT(t)
	_, cleanup, err := r.UnitRun(context.Background(), r.StartCmd, "start", sdunit.Run{Type: "notify"})
	cleanup()
	assert.ErrorContains(t, err, "does not run systemd")
	assert.Nil(t, r.unitNsenter(context.Background(), "check"))
}

// TestUnitRunRefusesAPropertyOmSets pins that a property om sets fails the
// start, as the validation refuses it.
func TestUnitRunRefusesAPropertyOmSets(t *testing.T) {
	defer func(f func() bool) { hasSystemd = f }(hasSystemd)
	hasSystemd = func() bool { return true }
	r := newUnitT(t)
	r.SystemdProperties = "PrivateTmp=yes User=root"
	_, cleanup, err := r.UnitRun(context.Background(), r.StartCmd, "start", sdunit.Run{Type: "notify"})
	cleanup()
	assert.ErrorContains(t, err, "User is refused")
}

func TestUnitStartTimeout(t *testing.T) {
	r := newUnitT(t)
	assert.Equal(t, sdunit.Property{Key: "TimeoutStartSec", Value: "infinity"}, r.UnitStartTimeout())
	d := 90 * time.Second
	r.StartTimeout = &d
	assert.Equal(t, sdunit.Property{Key: "TimeoutStartSec", Value: "90000ms"}, r.UnitStartTimeout())
}

func TestKeywordSystemdPropertiesValidates(t *testing.T) {
	require.NotNil(t, BaseKeywordSystemdProperties.Validate)
	assert.NoError(t, BaseKeywordSystemdProperties.Validate("ProtectSystem=strict ReadWritePaths=/srv/a /srv/b"))
	assert.Error(t, BaseKeywordSystemdProperties.Validate("ExecStartPre=/bin/sh"))
	assert.True(t, slices.Contains(BaseKeywords, &BaseKeywordSystemdProperties))
}

// TestNetRulesTurnUnitModeOn pins that net_rules makes the start command
// run in the unit, as systemd_properties does: systemd filters the
// units of the slice of the process group, and no process outside one.
func TestNetRulesTurnUnitModeOn(t *testing.T) {
	r := newUnitT(t)
	r.SystemdProperties = ""
	assert.False(t, r.UnitMode())

	r.Net.Rules = "allow node n2"
	assert.True(t, r.UnitMode())
	assert.Equal(t, "net_rules", r.unitKeywords())
	assert.Equal(t, "opensvc-ns.prod-svc.web-app.main.service", r.UnitName())

	r.Net.Rules = "deny any"
	assert.True(t, r.UnitMode())

	r.SystemdProperties = "PrivateTmp=yes"
	assert.Equal(t, "systemd_properties, net_rules", r.unitKeywords())

	r.Net.Rules = " \n"
	assert.True(t, r.UnitMode())
	assert.Equal(t, "systemd_properties", r.unitKeywords())
}

// TestUnitRunRefusesNetRulesItCanNotApply pins that a start whose filter
// could not be enforced is refused before anything is made, rather than run
// unfiltered: on a node without systemd, for a process group that is no
// slice or is delegated to a user, and for rules the grammar refuses.
func TestUnitRunRefusesNetRulesItCanNotApply(t *testing.T) {
	defer func(f func() bool) { hasSystemd = f }(hasSystemd)
	run := func(r *T) error {
		_, cleanup, err := r.UnitRun(context.Background(), r.StartCmd, "start", sdunit.Run{Type: "notify"})
		cleanup()
		return err
	}
	netT := func() *T {
		r := newUnitT(t)
		r.SystemdProperties = ""
		r.Net.Rules = "allow node n2"
		return r
	}

	hasSystemd = func() bool { return false }
	assert.EqualError(t, run(netT()), "net_rules: the node does not run systemd")

	hasSystemd = func() bool { return true }
	r := netT()
	r.SetPG(&pg.Config{})
	err := run(r)
	assert.ErrorIs(t, err, pg.ErrNoSlice)
	assert.EqualError(t, err, "netwarden: net_rules can not be applied: the process group is no systemd slice")

	r = netT()
	r.SetPG(r.GetPG().Delegated(pg.Delegation{Root: "/user.slice/user-1001.slice/user@1001.service", UID: 1001, GID: 1001}))
	assert.ErrorIs(t, run(r), pg.ErrDelegated)

	r = netT()
	r.Net.Rules = "allow node n2\ndeny node n3 port 22 proto tcp"
	assert.ErrorContains(t, run(r), "netwarden: net_rules line 2: \"deny node n3 port 22 proto tcp\": port 22: the cgroup filter")

	r = netT()
	r.Net.Rules = "allow address from ./svc/acl key k"
	assert.ErrorContains(t, run(r), "./svc/acl is no datastore, a cfg or a sec")
}

// The command of an app, as the one of a task run on the node, resolves
// through the resolver of the node: its filter is given no nameservers of
// its own, so it opens the ones the resolv.conf of the node names, and not
// the ones of cluster.dns a container is given.
func TestNetTargetNamesNoNameservers(t *testing.T) {
	r := newUnitT(t)
	r.Net.Rules = "allow node n2"
	assert.Nil(t, r.NetTarget().DNS)
}

// TestUnitHoldsFollowsTheUnitRunning pins what a stop ends: the unit of a
// resource whose start runs in one, and the unit still running a command
// started in one under a configuration since changed, as rules the daemon
// took away from the running resource, whatever the configuration says
// now. A resource whose unit runs nothing, or a node without systemd, ends
// none.
func TestUnitHoldsFollowsTheUnitRunning(t *testing.T) {
	defer func(f func() bool) { hasSystemd = f }(hasSystemd)
	defer func(f func(context.Context, string) (sdunit.State, error)) { unitState = f }(unitState)
	hasSystemd = func() bool { return true }
	active := false
	unitState = func(context.Context, string) (sdunit.State, error) {
		if active {
			return sdunit.State{ActiveState: "active"}, nil
		}
		return sdunit.State{ActiveState: "inactive"}, nil
	}
	ctx := context.Background()
	r := newUnitT(t)
	r.SystemdProperties = "PrivateTmp=yes"
	assert.True(t, r.UnitHolds(ctx), "the start runs in a unit")

	r.SystemdProperties = ""
	assert.False(t, r.UnitHolds(ctx), "no unit, and none running")
	active = true
	assert.True(t, r.UnitHolds(ctx), "a unit still running the command, its keywords gone")

	hasSystemd = func() bool { return false }
	assert.False(t, r.UnitHolds(ctx), "no systemd")
}
