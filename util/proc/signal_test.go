package proc

import (
	"os/exec"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A process is signaled only when the check, made once it is held, says it
// is still the one found.
func TestSignalIf(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	p := New(cmd.Process.Pid)

	sent, err := p.SignalIf(syscall.SIGTERM, func(T) bool { return false })
	require.NoError(t, err)
	assert.False(t, sent, "a process the check refuses is not signaled")
	require.NoError(t, cmd.Process.Signal(syscall.Signal(0)), "and still runs")

	var checked int
	sent, err = p.SignalIf(syscall.SIGTERM, func(q T) bool { checked = q.PID(); return true })
	require.NoError(t, err)
	assert.True(t, sent)
	assert.Equal(t, cmd.Process.Pid, checked, "the check is asked about the pid held")
	err = cmd.Wait()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminated")
}
