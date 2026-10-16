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
