package drbd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/command"
)

// A command failing on a device in use is run again, each attempt a new
// command: running the same one again failed with "command: already
// started", which hid the error of the command.
func TestRetryRunsANewCommandEachAttempt(t *testing.T) {
	saved := RetryDelay
	RetryDelay = 10 * time.Millisecond
	t.Cleanup(func() { RetryDelay = saved })
	count := filepath.Join(t.TempDir(), "count")
	script := `echo x >>` + count + `; [ $(wc -l <` + count + `) -ge 3 ] || exit 11`
	newCmd := func() *command.T {
		return command.New(command.WithName("/bin/sh"), command.WithArgs([]string{"-c", script}))
	}
	require.NoError(t, retry(newCmd))
	b, err := os.ReadFile(count)
	require.NoError(t, err)
	require.Equal(t, "x\nx\nx\n", string(b), "three attempts")
}

// A failure that is not a device in use is returned as it is, the first
// time.
func TestRetryReturnsTheErrorOfTheCommand(t *testing.T) {
	newCmd := func() *command.T {
		return command.New(command.WithName("/bin/sh"), command.WithArgs([]string{"-c", "exit 3"}))
	}
	err := retry(newCmd)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "already started")
}
