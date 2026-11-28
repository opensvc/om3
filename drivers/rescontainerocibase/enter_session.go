package rescontainerocibase

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// enterMarkerName is the environment variable marking the processes of a
// session entering a container.
const enterMarkerName = "OSVC_ENTER_SESSION"

var (
	// enterKillGrace bounds the wait for the processes of a session hung up
	// to end, before they are killed, and the wait for the client of the
	// exec after: within the grace a console gives a session it hangs up.
	enterKillGrace = 2 * time.Second

	// enterKill sends sig to the process pid, a variable for the tests.
	enterKill = func(pid int, sig syscall.Signal) error {
		return syscall.Kill(pid, sig)
	}

	// enterPoll is the delay between the looks for the processes of a
	// session left.
	enterPoll = 100 * time.Millisecond
)

// newEnterMarker returns the NAME=VALUE environment variable marking the
// processes of a new session entering a container.
func newEnterMarker() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("make the marker of the session: %w", err)
	}
	return enterMarkerName + "=" + hex.EncodeToString(b), nil
}

// endEnterSession ends the processes of the session marked by marker, in the
// pid namespace of the process pid of the container: they are hung up, as by
// their terminal, and the ones left after grace are killed.
func endEnterSession(pid int, marker string, grace time.Duration) {
	pids := enterSessionPIDs(pid, marker)
	if len(pids) == 0 {
		return
	}
	for _, p := range pids {
		_ = enterKill(p, syscall.SIGHUP)
	}
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if pids = enterSessionPIDs(pid, marker); len(pids) == 0 {
			return
		}
		time.Sleep(enterPoll)
	}
	for _, p := range enterSessionPIDs(pid, marker) {
		_ = enterKill(p, syscall.SIGKILL)
	}
}

// enterSessionPIDs returns the processes of the pid namespace of the process
// pid whose environment holds marker: the shell of the session and what it
// started. A process may rewrite its environment, which a process of the
// session would only do to stay out of it.
func enterSessionPIDs(pid int, marker string) []int {
	ns, err := os.Readlink(filepath.Join(procRoot, strconv.Itoa(pid), "ns", "pid"))
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil
	}
	want := []byte(marker)
	var l []int
	for _, entry := range entries {
		p, err := strconv.Atoi(entry.Name())
		if err != nil || p == pid {
			continue
		}
		dir := filepath.Join(procRoot, entry.Name())
		if theirs, err := os.Readlink(filepath.Join(dir, "ns", "pid")); err != nil || theirs != ns {
			continue
		}
		environ, err := os.ReadFile(filepath.Join(dir, "environ"))
		if err != nil {
			continue
		}
		for _, kv := range bytes.Split(environ, []byte{0}) {
			if bytes.Equal(kv, want) {
				l = append(l, p)
				break
			}
		}
	}
	return l
}
