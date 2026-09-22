package commoncmd

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Asking again is worth it only when every node answered that the work is
// still running. A node that failed to answer at all is an answer to report.
func TestOnlyStillRunningIsWorthAskingAgainFor(t *testing.T) {
	assert.False(t, IsStillRunning(nil))
	assert.True(t, IsStillRunning(fmt.Errorf("node1: %w", ErrStillRunning)))
	assert.True(t, IsStillRunning(errors.Join(
		fmt.Errorf("node1: %w", ErrStillRunning),
		fmt.Errorf("node2: %w", ErrStillRunning),
	)))
	assert.False(t, IsStillRunning(errors.Join(
		fmt.Errorf("node1: %w", ErrStillRunning),
		errors.New("node2: connection refused"),
	)))
	assert.False(t, IsStillRunning(errors.New("connection refused")))
}

// A wait command with no duration waits for as long as it takes, and one
// asked for a duration that is not positive is refused: it used to become a
// listing that reported success on work still running.
func TestTheWaitDurationIsReadOrRefused(t *testing.T) {
	newCmd := func(wait *time.Duration) *cobra.Command {
		cmd := &cobra.Command{Use: "wait"}
		cmd.Flags().DurationVar(wait, "duration", 0, "")
		return cmd
	}

	var wait time.Duration
	var unbounded bool
	cmd := newCmd(&wait)
	require.NoError(t, SetWait(cmd, &wait, &unbounded))
	assert.True(t, unbounded)
	assert.Equal(t, DefaultWait, wait)

	wait, unbounded = 0, false
	cmd = newCmd(&wait)
	require.NoError(t, cmd.ParseFlags([]string{"--duration", "5s"}))
	require.NoError(t, SetWait(cmd, &wait, &unbounded))
	assert.False(t, unbounded)
	assert.Equal(t, 5*time.Second, wait)

	wait, unbounded = 0, false
	cmd = newCmd(&wait)
	require.NoError(t, cmd.ParseFlags([]string{"--duration", "-1s"}))
	assert.Error(t, SetWait(cmd, &wait, &unbounded))

	wait, unbounded = 0, false
	cmd = newCmd(&wait)
	require.NoError(t, cmd.ParseFlags([]string{"--duration", "0s"}))
	assert.Error(t, SetWait(cmd, &wait, &unbounded), "a zero duration asked for is not a wait")
}
