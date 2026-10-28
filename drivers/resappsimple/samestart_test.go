package resappsimple

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/proc"
)

// A process found is the one held when it still started when it did, and a
// process the scan did not find, or one gone since, is not.
func TestSameStartAs(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	other := exec.Command("sleep", "60")
	require.NoError(t, other.Start())
	t.Cleanup(func() { _ = other.Process.Kill(); _ = other.Wait() })

	l := proc.NewList()
	l.AddPID(cmd.Process.Pid)
	same := sameStartAs(l)

	assert.True(t, same(proc.New(cmd.Process.Pid)), "the process found")
	assert.False(t, same(proc.New(other.Process.Pid)), "a process the scan did not find")

	require.NoError(t, cmd.Process.Kill())
	_ = cmd.Wait()
	assert.False(t, same(proc.New(cmd.Process.Pid)), "a process gone since")
}
