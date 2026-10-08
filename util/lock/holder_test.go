//go:build linux

package lock

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// flockHolderEnv makes the test binary a process holding the lock taken by
// Lock, for the tests about the holder a timeout names.
const flockHolderEnv = "OM_TEST_FLOCK_HOLDER"

func TestATimeoutNamesTheHolder(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.lock")
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), flockHolderEnv+"="+p)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "locked\n", line)

	_, err = Lock(p, 100*time.Millisecond, "test wait")
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "lock timeout exceeded after 100ms on "+p)
	assert.Contains(t, msg, fmt.Sprintf("held by pid %d (%s -test.run=^$)", cmd.Process.Pid, os.Args[0]))
	assert.Contains(t, msg, "for test hold since ")
	assert.Contains(t, msg, ", session ")
}

func TestAnErrorNotATimeoutIsKept(t *testing.T) {
	// A lock file in a directory that can not be made is no holder's
	// doing.
	p := filepath.Join(t.TempDir(), "file", "x.lock")
	require.NoError(t, os.WriteFile(filepath.Dir(p), nil, 0o600))
	_, err := Lock(p, 100*time.Millisecond, "test wait")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "held by")
}
