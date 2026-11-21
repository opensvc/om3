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
