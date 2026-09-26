//go:build linux

package pg

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/plog"
)

// testDelegation makes a group standing for the subtree systemd delegates to
// a user, offering the controllers given, and removes it and what the test
// made under it when the test ends.
func testDelegation(t *testing.T, name string, controllers ...string) Delegation {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("making a cgroup needs root")
	}
	if !isUnified() {
		t.Skip("no unified cgroup hierarchy")
	}
	// The group standing for user@<uid>.service is a child of a group
	// offering only the controllers asked for, the way user@.service is
	// offered cpu, memory and pids by a default systemd.
	top := "/omtest-deleg-" + name + ".slice"
	root := top + "/user.slice"
	require.NoError(t, os.Mkdir(filepath.Join(UnifiedPath(), top), 0755))
	l := make([]string, 0)
	for _, c := range controllers {
		l = append(l, "+"+c)
	}
	require.NoError(t, os.WriteFile(filepath.Join(UnifiedPath(), top, "cgroup.subtree_control"), []byte(strings.Join(l, " ")), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(UnifiedPath(), root), 0755))
	t.Cleanup(func() {
		// Remove leaf first.
		var dirs []string
		_ = filepath.Walk(filepath.Join(UnifiedPath(), top), func(path string, info os.FileInfo, err error) error {
			if err == nil && info.IsDir() {
				dirs = append(dirs, path)
			}
			return nil
		})
		for i := len(dirs) - 1; i >= 0; i-- {
			_ = os.Remove(dirs[i])
		}
	})
	return Delegation{Root: root, UID: 65534, GID: 65534}
}

func owner(t *testing.T, path string) (int, int) {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	st := info.Sys().(*syscall.Stat_t)
	return int(st.Uid), int(st.Gid)
}

// A group delegated to a user is made under the subtree systemd delegates to
// them, owned by them, so their systemd instance can make the group of a
// rootless container inside it. The cappings are written by om, in files the
// user is not given.
func TestADelegatedGroupIsMadeOwnedByTheUserAndCappedByOm(t *testing.T) {
	d := testDelegation(t, "owned", "cpu", "memory", "pids")
	c := Config{
		ID:        "/opensvc.slice/opensvc-svc.t.slice",
		CPUShares: "200",
		MemLimit:  "256m",
	}
	dc := c.Delegated(d)
	assert.Equal(t, d.Root+c.ID, dc.Path(), "the group lives under the delegated subtree")
	assert.Equal(t, c.ID, dc.ID, "and remains the same group of the same object")

	_, err := dc.ApplyProc(0)
	require.NoError(t, err)

	for _, rel := range []string{"/opensvc.slice", c.ID} {
		dir := filepath.Join(UnifiedPath(), d.Root, rel)
		uid, gid := owner(t, dir)
		assert.Equalf(t, [2]int{65534, 65534}, [2]int{uid, gid}, "%s is the user's", rel)
		for _, name := range delegationFiles {
			uid, gid := owner(t, filepath.Join(dir, name))
			assert.Equalf(t, [2]int{65534, 65534}, [2]int{uid, gid}, "%s/%s is the user's", rel, name)
		}
	}
	leaf := filepath.Join(UnifiedPath(), dc.Path())
	for file, expected := range map[string]string{
		"cpu.weight": "200",
		"memory.max": "268435456",
	} {
		b, err := os.ReadFile(filepath.Join(leaf, file))
		require.NoError(t, err)
		assert.Equalf(t, expected, strings.TrimSpace(string(b)), "%s is capped", file)
		uid, _ := owner(t, filepath.Join(leaf, file))
		assert.Equalf(t, 0, uid, "%s stays root's, so the user cannot lift it", file)
	}
	b, err := os.ReadFile(filepath.Join(leaf, "cgroup.subtree_control"))
	require.NoError(t, err)
	assert.Contains(t, string(b), "cpu", "the leaf offers its controllers to the group of the container")
}

// A capping whose controller the subtree is not delegated is said not to be
// applied, and the others hold: io under a default user@.service is the case.
func TestACappingOfAControllerNotDelegatedIsReportedNotFailed(t *testing.T) {
	d := testDelegation(t, "noio", "cpu", "memory")
	c := Config{
		ID:            "/opensvc.slice/opensvc-svc.t.slice",
		CPUShares:     "300",
		BlockIOWeight: "500",
	}
	dc := c.Delegated(d).WithLogger(plog.NewDefaultLogger())
	_, err := dc.ApplyProc(0)
	require.NoError(t, err, "a capping that cannot be applied does not fail the others")

	leaf := filepath.Join(UnifiedPath(), dc.Path())
	b, err := os.ReadFile(filepath.Join(leaf, "cpu.weight"))
	require.NoError(t, err)
	assert.Equal(t, "300", strings.TrimSpace(string(b)))
	_, err = os.Stat(filepath.Join(leaf, "io.weight"))
	assert.ErrorIs(t, err, os.ErrNotExist, "the io controller was not delegated")
}

// The subtree is made by the systemd instance of the user, so its absence
// says that instance is not running, and the error says so.
func TestAMissingDelegatedSubtreeNamesTheStoppedSystemdInstance(t *testing.T) {
	if !isUnified() {
		t.Skip("no unified cgroup hierarchy")
	}
	c := Config{ID: "/opensvc.slice/opensvc-svc.t.slice"}
	dc := c.Delegated(Delegation{Root: "/omtest-no-such.slice", UID: 1001, GID: 1001})
	_, err := dc.ApplyProc(0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the systemd instance of uid 1001 is not running")
}
