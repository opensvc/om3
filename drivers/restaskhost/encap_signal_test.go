package restaskhost

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/signalrelay"
)

// ctxCommander is a commander that runs until its context is cancelled, as
// the one of a kvm guest agent polls the guest until then.
type ctxCommander struct{ ctx context.Context }

func (c *ctxCommander) CombinedOutput() ([]byte, error) {
	<-c.ctx.Done()
	return nil, errors.New("cancelled")
}
func (c *ctxCommander) Run() error                         { return nil }
func (c *ctxCommander) Start() error                       { return nil }
func (c *ctxCommander) StderrPipe() (io.ReadCloser, error) { return nil, nil }
func (c *ctxCommander) Wait() error                        { return nil }

// The signal the run receives stops the command run in the container, a
// process or a commander stopped by its context, and is the end of the run.
// A signal received after the command ended is not.
func TestTheSignalOfTheRunReachesTheCommandInTheContainer(t *testing.T) {
	ctx, stop := signalrelay.WithSignals(context.Background(), syscall.SIGUSR2)
	defer stop()
	r := &T{}

	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = syscall.Kill(os.Getpid(), syscall.SIGUSR2)
	}()
	cmdCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	began := time.Now()
	_, signaled, err := r.runEncapCmd(ctx, exec.CommandContext(cmdCtx, "/bin/sleep", "30"), cancel)
	require.Error(t, err)
	assert.Equal(t, syscall.SIGUSR2, signaled, "the process got the signal")
	assert.Less(t, time.Since(began), 10*time.Second)

	// The relay holds the signal received: a command started after it is
	// signaled at once, a commander stopped by its context too.
	cmdCtx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()
	_, signaled, err = r.runEncapCmd(ctx, &ctxCommander{ctx: cmdCtx2}, cancel2)
	require.Error(t, err)
	assert.Equal(t, syscall.SIGUSR2, signaled, "the commander was cancelled")
}

// A command that ended on its own ends the run as it exited, a signal
// received after it reaching nothing.
func TestALateSignalIsNotTheEndOfTheRun(t *testing.T) {
	ctx, stop := signalrelay.WithSignals(context.Background(), syscall.SIGUSR1)
	defer stop()
	r := &T{}
	cmdCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	_, signaled, err := r.runEncapCmd(ctx, exec.CommandContext(cmdCtx, "/bin/true"), cancel)
	require.NoError(t, err)
	assert.Nil(t, signaled)
}
