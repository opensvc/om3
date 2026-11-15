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
