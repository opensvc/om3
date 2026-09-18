package command_test

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/command"
)

// A command that writes and exits is waited for the whole of what it wrote.
// The delay is a bound on the pathological case and must cost this nothing.
func TestWaitDelayDoesNotTruncateOutput(t *testing.T) {
	lines := make([]string, 0)
	cmd := command.New(
		command.WithName("/bin/sh"),
		command.WithVarArgs("-c", "for i in $(seq 1 500); do echo line $i; done"),
		command.WithOnStdoutLine(func(s string) { lines = append(lines, s) }),
		command.WithBufferedStdout(),
	)
	require.NoError(t, cmd.Run())
	assert.Len(t, lines, 500, "every line the command wrote is read")
	assert.Equal(t, "line 1", lines[0])
	assert.Equal(t, "line 500", lines[499])
	assert.Equal(t, 0, cmd.ExitCode())
}

// A command that hands its output pipe to something outliving it used to be
// waited on for ever: the pipe reaches EOF when the last holder of its write
// end is gone, and the holder here is still asleep. The delay bounds it.
func TestWaitDelayBoundsAnOrphanHoldingTheOutput(t *testing.T) {
	cmd := command.New(
		command.WithName("/bin/sh"),
		// The sh exits at once. The sleep it leaves behind inherits the
		// write end of the pipe, and holds it for a minute.
		command.WithVarArgs("-c", "echo started; sleep 60 &"),
		command.WithBufferedStdout(),
		command.WithWaitDelay(time.Second),
	)
	begin := time.Now()
	err := cmd.Run()
	elapsed := time.Since(begin)

	assert.Less(t, elapsed, 20*time.Second, "bounded by the delay, not by the orphan")
	assert.GreaterOrEqual(t, elapsed, time.Second, "and not cut before the delay")

	// The command did what it was asked, so it is not failed for what it
	// left behind, and what it wrote before exiting is still read.
	assert.NoError(t, err)
	assert.Equal(t, 0, cmd.ExitCode())
	assert.True(t, strings.HasPrefix(string(cmd.Stdout()), "started"), "got %q", string(cmd.Stdout()))
}

// The exit code of a command that failed is reported whatever its pipes did.
func TestWaitDelayKeepsTheExitCode(t *testing.T) {
	cmd := command.New(
		command.WithName("/bin/sh"),
		command.WithVarArgs("-c", "echo out; exit 3"),
		command.WithBufferedStdout(),
	)
	err := cmd.Run()
	require.Error(t, err)
	var exitErr *exec.ExitError
	_ = exitErr
	assert.Equal(t, 3, cmd.ExitCode())
	assert.Equal(t, "out\n", string(cmd.Stdout()))
}
