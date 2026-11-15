package rescontainerocibase

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeNsenter has nsenterArgs read the help given, and the namespaces of
// the agent and of the process 42 from a fake proc tree, the process having
// those of its own given.
func fakeNsenter(t *testing.T, help string, own ...string) {
	t.Helper()
	root := t.TempDir()
	for _, pid := range []string{"self", "42"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, pid, "ns"), 0o755))
	}
	for _, ns := range []string{"mnt", "uts", "ipc", "net", "pid", "user", "cgroup"} {
		require.NoError(t, os.Symlink(ns+":[1]", filepath.Join(root, "self", "ns", ns)))
		target := ns + ":[1]"
		for _, o := range own {
			if o == ns {
				target = ns + ":[2]"
			}
		}
		require.NoError(t, os.Symlink(target, filepath.Join(root, "42", "ns", ns)))
	}
	savedHelp, savedRoot := nsenterHelp, procRoot
	nsenterHelp = func() string { return help }
	procRoot = root
	nsenterOptionsOnce = sync.Once{}
	t.Cleanup(func() {
		nsenterHelp, procRoot = savedHelp, savedRoot
		nsenterOptionsOnce = sync.Once{}
	})
}

// The namespaces of a container are entered with --all where nsenter knows
// it, and named where it does not, as on rhel7: the user and cgroup ones only
// where the container has its own, entering one's own user namespace failing
// (card 305).
func TestNsenterArgs(t *testing.T) {
	recent := " -a, --all              enter all namespaces\n -C, --cgroup[=<file>]  enter cgroup namespace\n"
	rhel7 := " -m, --mount[=<file>]   enter mount namespace\n -U, --user[=<file>]    enter user namespace\n"

	fakeNsenter(t, recent, "mnt", "pid")
	assert.Equal(t, "nsenter -t 42 --all", strings.Join(nsenterArgs(42), " "))

	fakeNsenter(t, rhel7, "mnt", "pid")
	assert.Equal(t, "nsenter -t 42 -m -u -i -n -p", strings.Join(nsenterArgs(42), " "), "the user namespace of the agent")

	fakeNsenter(t, rhel7, "mnt", "pid", "user", "cgroup")
	assert.Equal(t, "nsenter -t 42 -m -u -i -n -p -U", strings.Join(nsenterArgs(42), " "), "an nsenter of no cgroup option")

	fakeNsenter(t, rhel7+" -C, --cgroup[=<file>]  enter cgroup namespace\n", "user", "cgroup")
	assert.Equal(t, "nsenter -t 42 -m -u -i -n -p -U -C", strings.Join(nsenterArgs(42), " "))
}

// containerRoot makes the root of the files of the container of the process
// 42 under a fake proc tree, and returns it.
func containerRoot(t *testing.T) string {
	t.Helper()
	proc := t.TempDir()
	root := filepath.Join(proc, "42", "root")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "bin"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "usr", "bin"), 0o755))
	saved := procRoot
	procRoot = proc
	t.Cleanup(func() { procRoot = saved })
	return root
}

// The shell of a container is looked for in its files, its links resolved
// as the container sees them: an absolute link leads into the container, a
// relative one from its directory, a loop to nothing, and a file not
// executable is no shell. The shell was run to see whether it existed, which
// the nsenter of rhel7 answers by an exit code it also gives other failures,
// and /bin/bash was taken in an image of /bin/sh alone (card 305).
func TestContainerHasExecutable(t *testing.T) {
	root := containerRoot(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "bin", "busybox"), []byte("#!"), 0o755))
	require.NoError(t, os.Symlink("/bin/busybox", filepath.Join(root, "bin", "sh")))
	require.NoError(t, os.Symlink("../../bin/busybox", filepath.Join(root, "usr", "bin", "ash")))
	require.NoError(t, os.Symlink("/usr/bin/loop2", filepath.Join(root, "usr", "bin", "loop1")))
	require.NoError(t, os.Symlink("/usr/bin/loop1", filepath.Join(root, "usr", "bin", "loop2")))
	require.NoError(t, os.WriteFile(filepath.Join(root, "bin", "notexec"), []byte("x"), 0o644))
	require.NoError(t, os.Symlink("/nonexistent-on-the-node-too/sh", filepath.Join(root, "bin", "dangling")))
	require.NoError(t, os.Symlink("/usr", filepath.Join(root, "linkdir")))

	assert.True(t, containerHasExecutable(42, "/bin/sh"), "an absolute link resolved in the container")
	assert.True(t, containerHasExecutable(42, "/usr/bin/ash"), "a relative link")
	assert.True(t, containerHasExecutable(42, "/linkdir/../bin/busybox"), "a link to a directory, then ..")
	assert.False(t, containerHasExecutable(42, "/bin/bash"), "missing")
	assert.False(t, containerHasExecutable(42, "/usr/bin/loop1"), "a loop")
	assert.False(t, containerHasExecutable(42, "/bin/notexec"), "not executable")
	assert.False(t, containerHasExecutable(42, "/bin/dangling"), "a dangling link")
	assert.False(t, containerHasExecutable(42, "/bin"), "a directory")
	assert.False(t, containerHasExecutable(43, "/bin/sh"), "no such process")
}

// An absolute link of the container is not resolved on the node: a container
// whose /bin/sh leads to a file the node has and the container has not, has
// no shell.
func TestContainerLinksAreNotResolvedOnTheNode(t *testing.T) {
	root := containerRoot(t)
	nodeFile := filepath.Join(t.TempDir(), "sh")
	require.NoError(t, os.WriteFile(nodeFile, []byte("#!"), 0o755))
	require.NoError(t, os.Symlink(nodeFile, filepath.Join(root, "bin", "sh")))
	assert.False(t, containerHasExecutable(42, "/bin/sh"))
}
