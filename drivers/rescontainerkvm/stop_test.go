package rescontainerkvm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/drivers/rescontainer"
	"github.com/opensvc/om3/v3/util/plog"
)

// fakeVirsh puts a virsh in front of the PATH whose domain is in the state of
// a file, and whose shutdown and destroy act as the script given for them
// says, the calls recorded.
func fakeVirsh(t *testing.T, state, shutdown, destroy string) (stateFile, callsFile string) {
	dir := t.TempDir()
	stateFile = filepath.Join(dir, "state")
	callsFile = filepath.Join(dir, "calls")
	require.NoError(t, os.WriteFile(stateFile, []byte(state), 0644))
	script := `#!/bin/sh
echo "$1" >> ` + callsFile + `
case "$1" in
dominfo) echo "Name:           $2"; echo "State:          $(cat ` + stateFile + `)";;
shutdown) ` + shutdown + `;;
destroy) ` + destroy + `;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "virsh"), []byte(script), 0755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	return stateFile, callsFile
}

func newStopTestT(t *testing.T) *T {
	r := &T{}
	r.Name = "vm1"
	timeout := time.Second
	r.StopTimeout = &timeout
	r.SetLoggerForTest(plog.NewDefaultLogger())
	return r
}

func calls(t *testing.T, callsFile string) string {
	b, err := os.ReadFile(callsFile)
	require.NoError(t, err)
	return strings.Join(strings.Fields(string(b)), " ")
}

// The guest powers off as the stop timeout expires: the destroy then fails on
// a domain that is not running, and the stop has done what it was asked.
func TestContainerStopDestroyFailingOnADomainDown(t *testing.T) {
	stateFile, callsFile := fakeVirsh(t, "running", "true",
		`echo "shut off" > STATE; echo "error: Requested operation is not valid: domain is not running" >&2; exit 1`)
	rewriteScript(t, stateFile)
	require.NoError(t, newStopTestT(t).containerStop(context.Background()))
	assert.Contains(t, calls(t, callsFile), "destroy")
}

// The guest powers off between the read of its state and the shutdown.
func TestContainerStopShutdownFailingOnADomainDown(t *testing.T) {
	stateFile, callsFile := fakeVirsh(t, "running",
		`echo "shut off" > STATE; echo "error: domain is not running" >&2; exit 1`, "exit 1")
	rewriteScript(t, stateFile)
	require.NoError(t, newStopTestT(t).containerStop(context.Background()))
	assert.NotContains(t, calls(t, callsFile), "destroy")
}

// A destroy failing on a domain still running is a failure of the stop.
func TestContainerStopDestroyFailingOnADomainUp(t *testing.T) {
	_, callsFile := fakeVirsh(t, "running", "true", `echo "error: failed" >&2; exit 1`)
	assert.Error(t, newStopTestT(t).containerStop(context.Background()))
	assert.Contains(t, calls(t, callsFile), "destroy")
}

// rewriteScript points the STATE placeholder of the shutdown and destroy
// scripts to the state file.
func rewriteScript(t *testing.T, stateFile string) {
	p := filepath.Join(filepath.Dir(stateFile), "virsh")
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(p, []byte(strings.ReplaceAll(string(b), "STATE", stateFile)), 0755))
}

// shortKillMargin makes the margin of the destroy fit a test.
func shortKillMargin(t *testing.T) {
	margin := rescontainer.StopKillMargin
	rescontainer.StopKillMargin = 300 * time.Millisecond
	t.Cleanup(func() { rescontainer.StopKillMargin = margin })
}

// The guest ignores the shutdown, and the deadline of the action is the stop
// timeout of the container, as when it inherits DEFAULT.stop_timeout, or
// comes first: the wait leaves the destroy time to end the stop within the
// deadline. The wait ended with the deadline, and the destroy failed on the
// context expired, the domain left running.
func TestContainerStopDestroysWithinTheDeadline(t *testing.T) {
	shortKillMargin(t)
	for name, stopTimeout := range map[string]time.Duration{
		"the deadline as the stop timeout":     1500 * time.Millisecond,
		"the deadline before the stop timeout": 10 * time.Second,
	} {
		t.Run(name, func(t *testing.T) {
			stateFile, callsFile := fakeVirsh(t, "running", "true", `echo "shut off" > STATE`)
			rewriteScript(t, stateFile)
			r := newStopTestT(t)
			r.StopTimeout = &stopTimeout
			ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
			defer cancel()
			require.NoError(t, r.containerStop(ctx))
			assert.NoError(t, ctx.Err(), "the stop ends within the deadline")
			assert.Contains(t, calls(t, callsFile), "shutdown")
			assert.Contains(t, calls(t, callsFile), "destroy")
		})
	}
}

// A domain in a state no shutdown applies to, as a qemu "in shutdown" not
// ending its power off, or "pmsuspended", is waited for down, then
// destroyed, as v2 did: the stop skipped it, and reported a domain up
// stopped.
func TestContainerStopWaitsThenDestroysTheOtherStates(t *testing.T) {
	for _, state := range []string{"in shutdown", "pmsuspended"} {
		t.Run(state, func(t *testing.T) {
			stateFile, callsFile := fakeVirsh(t, state, "exit 1", `echo "shut off" > STATE`)
			rewriteScript(t, stateFile)
			require.NoError(t, newStopTestT(t).containerStop(context.Background()))
			assert.Equal(t, "dominfo", strings.Fields(calls(t, callsFile))[0])
			assert.NotContains(t, calls(t, callsFile), "shutdown", "no shutdown applies")
			assert.Contains(t, calls(t, callsFile), "destroy")
		})
	}
}

// A domain "in shutdown" powering off within the stop timeout is not
// destroyed.
func TestContainerStopWaitsForADomainInShutdown(t *testing.T) {
	stateFile, callsFile := fakeVirsh(t, "in shutdown", "exit 1", "exit 1")
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = os.WriteFile(stateFile, []byte("shut off"), 0644)
	}()
	require.NoError(t, newStopTestT(t).containerStop(context.Background()))
	assert.NotContains(t, calls(t, callsFile), "destroy")
}

// A domain in a state the stop does not know, not going down, and whose
// destroy fails, is a failure of the stop, not a success with it up.
func TestContainerStopFailsOnAnUnknownStateLeftUp(t *testing.T) {
	_, callsFile := fakeVirsh(t, "unknown", "exit 1", `echo "error: failed" >&2; exit 1`)
	assert.Error(t, newStopTestT(t).containerStop(context.Background()))
	assert.Contains(t, calls(t, callsFile), "destroy")
}
