//go:build linux

package pg

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
)

// fakeNode stands for the cgroup mount and the systemd control directories
// of a node, in a temporary directory, and records the systemctl commands
// instead of running them.
type fakeNode struct {
	dir   string
	calls []string
}

func newFakeNode(t *testing.T, unified bool) *fakeNode {
	t.Helper()
	n := &fakeNode{dir: t.TempDir()}
	savedMount, savedUnified, savedHasSystemd := cgroupMount, isUnified, hasSystemd
	savedSystemControlDirs, savedUserControlDir, savedSystemctl := systemControlDirs, userControlDir, systemctl
	t.Cleanup(func() {
		cgroupMount, isUnified, hasSystemd = savedMount, savedUnified, savedHasSystemd
		systemControlDirs, userControlDir, systemctl = savedSystemControlDirs, savedUserControlDir, savedSystemctl
	})
	cgroupMount = filepath.Join(n.dir, "cgroup")
	require.NoError(t, os.MkdirAll(cgroupMount, 0755))
	if unified {
		require.NoError(t, os.WriteFile(filepath.Join(cgroupMount, "cgroup.procs"), nil, 0644))
	}
	isUnified = func() bool { return unified }
	hasSystemd = func() bool { return true }
	systemControlDirs = []string{
		filepath.Join(n.dir, "run/systemd/system.control"),
		filepath.Join(n.dir, "etc/systemd/system.control"),
	}
	userControlDir = func(uid int) string {
		return filepath.Join(n.dir, "run/user", strconv.Itoa(uid), "systemd/user.control")
	}
	systemctl = func(opts ...funcopt.O) *command.T {
		cmd := command.New(opts...)
		n.calls = append(n.calls, strings.Join(cmd.Cmd().Args[1:], " "))
		return command.New(command.WithName("true"))
	}
	return n
}

// mkdirs makes the groups, relative to base.
func mkdirs(t *testing.T, base string, groups ...string) {
	t.Helper()
	for _, g := range groups {
		require.NoError(t, os.MkdirAll(filepath.Join(base, g), 0755))
	}
}

// busy makes a group that cannot be removed, as one still holding a process:
// a directory with a file is not removed by rmdir either.
func busy(t *testing.T, base, group string) {
	t.Helper()
	mkdirs(t, base, group)
	require.NoError(t, os.WriteFile(filepath.Join(base, group, "cgroup.procs"), []byte("1234"), 0644))
}

// dropIns makes the drop-in directories systemctl set-property writes for
// the units.
func dropIns(t *testing.T, dir string, units ...string) {
	t.Helper()
	for _, u := range units {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, u+".d"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, u+".d", "50-MemoryMax.conf"), []byte("[Slice]\nMemoryMax=268435456\n"), 0644))
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

const (
	testNodeSlice = "/opensvc.slice"
	testNsSlice   = testNodeSlice + "/opensvc-ns.t.slice"
	testObjSlice  = testNsSlice + "/opensvc-ns.t-svc.s.slice"
	testObjUnit   = "opensvc-ns.t-svc.s.slice"
	testAppSlice  = testObjSlice + "/opensvc-ns.t-svc.s-app.1.slice"
	testSubSlice  = testObjSlice + "/opensvc-ns.t-svc.s-subset.g1.slice"
	testSubApp    = testSubSlice + "/opensvc-ns.t-svc.s-subset.g1-app.2.slice"
	testSibling   = testNsSlice + "/opensvc-ns.t-svc.s2.slice"
)

// The stop of an object removes its groups, those of its subsets and
// resources included, which the stop never registered, and has systemd forget
// the properties of their slices. The groups of the namespace and of the
// node, and the ones of another object, are left alone.
func TestRemoveTreeRemovesTheGroupsOfTheObjectAndRevertsTheirSlices(t *testing.T) {
	n := newFakeNode(t, true)
	mkdirs(t, cgroupMount, testAppSlice, testSubApp, testSibling)
	runtimeDir := systemControlDirs[0]
	dropIns(t, runtimeDir,
		"opensvc.slice",
		"opensvc-ns.t.slice",
		testObjUnit,
		"opensvc-ns.t-svc.s-app.1.slice",
		"opensvc-ns.t-svc.s-subset.g1-app.2.slice",
		"opensvc-ns.t-svc.s2.slice",
	)
	// A slice whose group is gone already, as after a manual rmdir.
	dropIns(t, systemControlDirs[1], "opensvc-ns.t-svc.s-app.3.slice")

	require.NoError(t, Config{ID: testObjSlice}.RemoveTree())

	for _, g := range []string{testObjSlice, testAppSlice, testSubSlice, testSubApp} {
		assert.Falsef(t, exists(filepath.Join(cgroupMount, g)), "%s is removed", g)
	}
	for _, g := range []string{testNodeSlice, testNsSlice, testSibling} {
		assert.Truef(t, exists(filepath.Join(cgroupMount, g)), "%s is not the object's", g)
	}
	assert.Equal(t, []string{
		"revert" +
			" opensvc-ns.t-svc.s-app.1.slice" +
			" opensvc-ns.t-svc.s-app.3.slice" +
			" opensvc-ns.t-svc.s-subset.g1-app.2.slice" +
			" opensvc-ns.t-svc.s.slice",
	}, n.calls, "one revert of the slices of the object, not of the namespace, the node or another object")
}

// A group still holding a process stays, and so do the groups above it and
// the properties of their slices: the process keeps running capped as it was.
func TestRemoveTreeKeepsABusyGroupWithItsProperties(t *testing.T) {
	n := newFakeNode(t, true)
	mkdirs(t, cgroupMount, testAppSlice)
	busy(t, cgroupMount, testSubApp)
	dropIns(t, systemControlDirs[0],
		testObjUnit,
		"opensvc-ns.t-svc.s-app.1.slice",
		"opensvc-ns.t-svc.s-subset.g1.slice",
		"opensvc-ns.t-svc.s-subset.g1-app.2.slice",
	)

	require.NoError(t, Config{ID: testObjSlice}.RemoveTree())

	assert.False(t, exists(filepath.Join(cgroupMount, testAppSlice)), "the empty group is removed")
	for _, g := range []string{testSubApp, testSubSlice, testObjSlice} {
		assert.Truef(t, exists(filepath.Join(cgroupMount, g)), "%s holds a process, or a group that does", g)
	}
	assert.Equal(t, []string{"revert opensvc-ns.t-svc.s-app.1.slice"}, n.calls, "only the slice whose group is gone")
}

// On the v1 hierarchy, the groups are removed from every hierarchy mounted,
// and a link to one, as cpu to cpu,cpuacct, is not walked twice.
func TestRemoveTreeRemovesTheGroupsFromEveryV1Hierarchy(t *testing.T) {
	n := newFakeNode(t, false)
	for _, h := range []string{"memory", "cpu,cpuacct", "cpuset", "pids", "systemd", "unified"} {
		mkdirs(t, filepath.Join(cgroupMount, h), testAppSlice, testSibling)
	}
	require.NoError(t, os.Symlink("cpu,cpuacct", filepath.Join(cgroupMount, "cpu")))

	require.NoError(t, Config{ID: testObjSlice}.RemoveTree())

	for _, h := range []string{"memory", "cpu,cpuacct", "cpuset", "pids", "systemd", "unified"} {
		assert.Falsef(t, exists(filepath.Join(cgroupMount, h, testObjSlice)), "%s: the group is removed", h)
		assert.Truef(t, exists(filepath.Join(cgroupMount, h, testSibling)), "%s: the other object is left alone", h)
	}
	assert.Empty(t, n.calls, "no property was set, none is reverted")
}

// The groups of a rootless container are in the tree systemd delegates to its
// user, and the properties of their slices are the systemd instance of that
// user's: they are removed and reverted there, through that instance.
func TestRemoveTreeRemovesTheGroupsDelegatedToAUser(t *testing.T) {
	n := newFakeNode(t, true)
	root := "/user.slice/user-7.slice/user@7.service"
	mkdirs(t, cgroupMount, testSibling)
	mkdirs(t, filepath.Join(cgroupMount, root), testAppSlice)
	dropIns(t, userControlDir(7), testObjUnit, "opensvc-ns.t-svc.s-app.1.slice", "opensvc-ns.t.slice")

	require.NoError(t, Config{ID: testObjSlice}.RemoveTree())

	assert.False(t, exists(filepath.Join(cgroupMount, root, testObjSlice)), "the delegated group is removed")
	assert.True(t, exists(filepath.Join(cgroupMount, root, testNsSlice)), "the copy of the namespace group is left alone")
	assert.Equal(t, []string{"--user revert opensvc-ns.t-svc.s-app.1.slice opensvc-ns.t-svc.s.slice"}, n.calls)
}

// Nothing to remove is not a failure, and runs no systemctl.
func TestRemoveTreeOfAnObjectNeverCappedDoesNothing(t *testing.T) {
	n := newFakeNode(t, true)
	mkdirs(t, cgroupMount, testNsSlice)
	require.NoError(t, Config{ID: testObjSlice}.RemoveTree())
	assert.Empty(t, n.calls)
	assert.True(t, exists(filepath.Join(cgroupMount, testNsSlice)))
}

func TestRemoveTreeRefusesTheRootGroup(t *testing.T) {
	newFakeNode(t, true)
	assert.Error(t, Config{ID: "/"}.RemoveTree())
	assert.Error(t, Config{}.RemoveTree())
	assert.True(t, exists(cgroupMount))
}
