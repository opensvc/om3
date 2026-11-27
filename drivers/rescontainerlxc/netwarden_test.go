package rescontainerlxc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/drivers/shared/netwarden"
	"github.com/opensvc/om3/v3/util/pg"
)

const (
	testSlicePG = "/opensvc.slice/opensvc-svc.web.slice/opensvc-svc.web-container.1.slice"
	testScope   = "opensvc-svc.web-container.1-lxc.scope"
)

// node sets what the node is, for the test: running systemd or not, on the
// unified hierarchy or not, with a lxc taking lxc.cgroup.dir or not.
func node(t *testing.T, sd, unified, cgroupDir bool) {
	t.Helper()
	savedSD, savedUnified, savedCgroupDir := hasSystemd, isUnified, hasCgroupDir
	t.Cleanup(func() { hasSystemd, isUnified, hasCgroupDir = savedSD, savedUnified, savedCgroupDir })
	hasSystemd = func() bool { return sd }
	isUnified = func() bool { return unified }
	hasCgroupDir = func() bool { return cgroupDir }
}

func newLxc(t *testing.T, pgID string) *T {
	t.Helper()
	r := New().(*T)
	require.NoError(t, r.SetRID("container#1"))
	r.Name = "c1"
	if pgID != "" {
		r.SetPG(&pg.Config{ID: pgID})
	}
	return r
}

// A container is started in a scope of the slice of its process group, lxc
// told to make its groups under the one of the scope, wherever a filter
// could be set, with rules or not: on a node running systemd on the unified
// hierarchy, for a group that is a slice. Elsewhere it is started as it was.
func TestStartCommand(t *testing.T) {
	lxcArgs := func(dir string) []string {
		return []string{"-d", "-n", "c1", "-o", "/var/tmp/svc_c1_lxc_start.log", "-s", "lxc.cgroup.dir=" + dir, "-f", "/srv/c1/config", "-P", "/srv"}
	}
	start := func(r *T) (string, []string) {
		return r.startCommand(r.lxcStartArgs("/srv/c1/config", []string{"-P", "/srv"}))
	}

	node(t, true, true, true)
	r := newLxc(t, testSlicePG)
	assert.Equal(t, testScope, r.scopeName())
	name, args := start(r)
	assert.Equal(t, "systemd-run", name)
	assert.Equal(t, append([]string{
		"--quiet",
		"--scope",
		"--slice=opensvc-svc.web-container.1.slice",
		"--unit=" + testScope,
		"--property=Delegate=yes",
		"--property=Before=opensvc-server.service",
		"--",
		"lxc-start",
	}, lxcArgs("opensvc.slice/opensvc-svc.web.slice/opensvc-svc.web-container.1.slice/"+testScope)...), args)

	// The rules change nothing of how it is started.
	r.Net = netwarden.T{Rules: "allow node n2"}
	_, withRules := start(r)
	assert.Equal(t, args, withRules)

	plain := lxcArgs("opensvc.slice/opensvc-svc.web.slice/opensvc-svc.web-container.1.slice")
	for what, set := range map[string]func(){
		"no systemd":           func() { node(t, false, true, true) },
		"a hybrid hierarchy":   func() { node(t, true, false, true) },
		"a group delegated":    func() { r.SetPG((&pg.Config{ID: testSlicePG}).Delegated(pg.Delegation{})) },
		"a group not a slice":  func() { r.SetPG(&pg.Config{ID: "/opensvc/svc.web/container.1"}) },
		"a group with no name": func() { r.SetPG(&pg.Config{}) },
	} {
		node(t, true, true, true)
		r = newLxc(t, testSlicePG)
		set()
		assert.Empty(t, r.scopeName(), what)
		name, args = start(r)
		assert.Equal(t, "lxc-start", name, what)
		if what == "a group not a slice" {
			assert.Equal(t, lxcArgs("opensvc/svc.web/container.1"), args, what)
		} else if what != "a group with no name" {
			assert.Equal(t, plain, args, what)
		}
	}

	// A lxc taking no lxc.cgroup.dir places the container by itself.
	node(t, true, true, false)
	r = newLxc(t, testSlicePG)
	name, args = start(r)
	assert.Equal(t, "lxc-start", name)
	assert.NotContains(t, args, "-s")

	// No process group.
	node(t, true, true, true)
	r = newLxc(t, "")
	name, _ = start(r)
	assert.Equal(t, "lxc-start", name)
}

// The start of a container with rules is refused where it would run
// unfiltered, before anything is set, and a container with none starts
// everywhere.
func TestNetApplyRefusesAContainerItCanNotFilter(t *testing.T) {
	node(t, true, true, true)
	r := newLxc(t, "")
	r.Net = netwarden.T{Rules: "allow node n2"}
	filtered, err := r.netApply()
	assert.False(t, filtered)
	assert.ErrorIs(t, err, pg.ErrNoSlice, "no process group")

	r = newLxc(t, "/opensvc/svc.web/container.1")
	r.Net = netwarden.T{Rules: "allow node n2"}
	_, err = r.netApply()
	assert.ErrorIs(t, err, pg.ErrNoSlice, "a process group that is no slice")

	r = newLxc(t, testSlicePG)
	r.Net = netwarden.T{Rules: "deny any port 22 proto tcp\nallow any"}
	_, err = r.netApply()
	assert.ErrorContains(t, err, "netwarden: net_rules line 1: \"deny any port 22 proto tcp\": port 22: the cgroup filter", "the filter of a cgroup knows no port")

	// No rule: nothing is refused, whatever the node.
	node(t, false, false, false)
	r = newLxc(t, "")
	filtered, err = r.netApply()
	assert.NoError(t, err)
	assert.False(t, filtered)
	assert.NoError(t, r.netIssue())
}

// A container running out of the scope of its slice, as one started by an
// agent that made none, is one no filter reaches: the daemon is told, for
// the rules it is given to wait for its next start.
func TestNetPlacementIssue(t *testing.T) {
	node(t, true, true, true)
	saved := cgroupOf
	t.Cleanup(func() { cgroupOf = saved })
	r := newLxc(t, testSlicePG)
	base := r.GetPG().Dir()

	cgroupOf = func(int) (string, error) { return base + "/" + testScope + "/lxc.payload.c1", nil }
	assert.NoError(t, r.netPlacementIssue(12))

	cgroupOf = func(int) (string, error) { return base + "/" + testScope + "/lxc.payload.c1/init.scope", nil }
	assert.NoError(t, r.netPlacementIssue(12), "the init of a system container moves to a group of its own")

	cgroupOf = func(int) (string, error) { return base + "/lxc.payload.c1", nil }
	assert.EqualError(t, r.netPlacementIssue(12),
		"netwarden: the container c1 runs in the cgroup "+base+"/lxc.payload.c1, in no systemd scope of the slice the filter is set on, as started by an agent making none: the rules apply at its next start")

	cgroupOf = func(int) (string, error) { return base + "/" + testScope + "x/lxc.payload.c1", nil }
	assert.Error(t, r.netPlacementIssue(12), "a scope of another name")

	cgroupOf = func(int) (string, error) { return "", errors.New("no such process") }
	assert.EqualError(t, r.netPlacementIssue(12), "netwarden: the cgroup of the process 12: no such process")
}

// The name of the scope is made free before a start: the scope of the
// container that just stopped is waited for, and one that failed forgotten.
// A container started in no scope asks systemd nothing.
func TestScopePrepare(t *testing.T) {
	savedWait, savedReset := scopeWaitInactive, scopeResetFailed
	t.Cleanup(func() { scopeWaitInactive, scopeResetFailed = savedWait, savedReset })
	var calls []string
	scopeWaitInactive = func(_ context.Context, unit string, _ time.Duration) (bool, error) {
		calls = append(calls, "wait "+unit)
		return true, nil
	}
	scopeResetFailed = func(_ context.Context, unit string) error {
		calls = append(calls, "reset "+unit)
		return nil
	}
	ctx := context.Background()

	node(t, true, true, true)
	r := newLxc(t, testSlicePG)
	require.NoError(t, r.scopePrepare(ctx))
	r.scopeEnd(ctx)
	assert.Equal(t, []string{"wait " + testScope, "reset " + testScope, "wait " + testScope}, calls)

	calls = nil
	node(t, false, true, true)
	require.NoError(t, r.scopePrepare(ctx))
	r.scopeEnd(ctx)
	assert.Empty(t, calls)
}
