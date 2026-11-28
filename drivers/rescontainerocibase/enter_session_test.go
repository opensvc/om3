package rescontainerocibase

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProc makes a proc tree of processes, each in the pid namespace and with
// the environment given, and returns a function removing a process from it.
func fakeProc(t *testing.T, procs map[int]struct {
	ns  string
	env []string
}) func(pid int) {
	t.Helper()
	root := t.TempDir()
	for pid, p := range procs {
		dir := filepath.Join(root, strconv.Itoa(pid))
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "ns"), 0o755))
		require.NoError(t, os.Symlink(p.ns, filepath.Join(dir, "ns", "pid")))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "environ"), []byte(strings.Join(p.env, "\x00")+"\x00"), 0o644))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(root, "self"), 0o755))
	saved := procRoot
	procRoot = root
	t.Cleanup(func() { procRoot = saved })
	return func(pid int) {
		_ = os.RemoveAll(filepath.Join(root, strconv.Itoa(pid)))
	}
}

type proc = struct {
	ns  string
	env []string
}

// The processes of a session are the ones of the pid namespace of the
// container carrying its marker exactly: not the main process of the
// container, not another session, not a process of the node carrying it.
func TestEnterSessionPIDs(t *testing.T) {
	const marker = enterMarkerName + "=abc"
	fakeProc(t, map[int]proc{
		10: {"pid:[2]", []string{"PATH=/bin"}},
		11: {"pid:[2]", []string{"PATH=/bin", marker}},
		12: {"pid:[2]", []string{marker, "TERM=xterm"}},
		13: {"pid:[2]", []string{enterMarkerName + "=abcd"}},
		14: {"pid:[1]", []string{marker}},
		15: {"pid:[2]", []string{"X=" + marker}},
	})
	pids := enterSessionPIDs(10, marker)
	sort.Ints(pids)
	assert.Equal(t, []int{11, 12}, pids)
	assert.Empty(t, enterSessionPIDs(99, marker), "no such container process")
}

// A session hung up has its processes hung up, and the ones left after the
// grace killed; a session of no process left sends nothing.
func TestEndEnterSession(t *testing.T) {
	const marker = enterMarkerName + "=abc"
	remove := fakeProc(t, map[int]proc{
		10: {"pid:[2]", nil},
		11: {"pid:[2]", []string{marker}},
		12: {"pid:[2]", []string{marker}},
	})
	var mu sync.Mutex
	var sent []string
	savedKill, savedPoll := enterKill, enterPoll
	enterPoll = 10 * time.Millisecond
	enterKill = func(pid int, sig syscall.Signal) error {
		mu.Lock()
		defer mu.Unlock()
		sent = append(sent, strconv.Itoa(pid)+":"+sig.String())
		if pid == 11 && sig == syscall.SIGHUP {
			// The shell ends on the hang up, its child ignores it.
			remove(11)
		}
		return nil
	}
	t.Cleanup(func() { enterKill, enterPoll = savedKill, savedPoll })

	endEnterSession(10, marker, 50*time.Millisecond)
	sort.Strings(sent[:2])
	assert.Equal(t, []string{"11:hangup", "12:hangup", "12:killed"}, sent)

	sent = nil
	remove(12)
	endEnterSession(10, marker, 50*time.Millisecond)
	assert.Empty(t, sent)
}

// The marker of a session is an environment variable of its own name and a
// value of its own.
func TestNewEnterMarker(t *testing.T) {
	a, err := newEnterMarker()
	require.NoError(t, err)
	b, err := newEnterMarker()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(a, enterMarkerName+"="))
	assert.Len(t, a, len(enterMarkerName)+1+32)
	assert.NotEqual(t, a, b)
}

// A shell entering a container is run by the exec of the engine, so that it
// is confined as the container is, with a terminal only when there is one,
// and the container name last but for the shell, as podman stops reading
// options at the first argument not one.
func TestEnterCmdArgs(t *testing.T) {
	ea := &ExecutorArg{BT: &BT{Name: "c1"}}
	assert.Equal(t,
		[]string{"exec", "-i", "-t", "--env", "OSVC_ENTER_SESSION=x", "c1", "/bin/sh"},
		ea.EnterCmdArgs("/bin/sh", "OSVC_ENTER_SESSION=x", true))
	assert.Equal(t,
		[]string{"exec", "-i", "--env", "OSVC_ENTER_SESSION=x", "c1", "/bin/bash"},
		ea.EnterCmdArgs("/bin/bash", "OSVC_ENTER_SESSION=x", false))
}
