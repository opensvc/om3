package command

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/signalrelay"
)

// A command run with the relay of the signal the process receives gets it,
// ends as the signal makes it, and is waited on: the process is not ended,
// and the command does not run on.
func TestACommandGetsTheSignalRelayed(t *testing.T) {
	ctx, stop := signalrelay.WithSignals(context.Background(), syscall.SIGUSR2)
	defer stop()
	cmd := New(
		WithName("/bin/sh"),
		WithVarArgs("-c", `trap "exit 7" USR2; sleep 30 & wait`),
		WithSignalRelay(ctx),
		WithProcessGroup(),
		WithIgnoredExitCodes(),
	)
	require.NoError(t, cmd.Start())
	time.Sleep(200 * time.Millisecond)
	began := time.Now()
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGUSR2))
	_ = cmd.Wait()
	assert.Less(t, time.Since(began), 10*time.Second, "the command ended on the signal")
	assert.Equal(t, 7, cmd.ExitCode(), "the command ended as its trap made it")
}
