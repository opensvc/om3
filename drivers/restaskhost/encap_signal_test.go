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

// signalCommander is a commander signaling its command, as the one of a kvm
// guest agent runs a kill in the guest: the command then ends as the signal
// makes it.
type signalCommander struct {
	ended chan int
}

func (c *signalCommander) Signal(sig os.Signal) error {
	c.ended <- 128 + int(sig.(syscall.Signal))
	return nil
}
func (c *signalCommander) CombinedOutput() ([]byte, error) {
	code := <-c.ended
	return nil, &exec.ExitError{ProcessState: nil, Stderr: []byte{byte(code)}}
}
func (c *signalCommander) Run() error                         { return nil }
func (c *signalCommander) Start() error                       { return nil }
func (c *signalCommander) StderrPipe() (io.ReadCloser, error) { return nil, nil }
func (c *signalCommander) Wait() error                        { return nil }

// A commander signaling its command gets the signal of the run, and is
// waited on rather than cancelled.
func TestACommanderSignalingItsCommandGetsTheSignal(t *testing.T) {
	ctx, stop := signalrelay.WithSignals(context.Background(), syscall.SIGWINCH)
	defer stop()
	r := &T{}
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = syscall.Kill(os.Getpid(), syscall.SIGWINCH)
	}()
	cancelled := false
	_, signaled, err := r.runEncapCmd(ctx, &signalCommander{ended: make(chan int, 1)}, func() { cancelled = true })
	require.Error(t, err)
	assert.Equal(t, syscall.SIGWINCH, signaled)
	assert.False(t, cancelled, "waited on, not cancelled")
}

// exitCodeError is the error of a command exiting with a code.
type exitCodeError int

func (e exitCodeError) Error() string { return "exit status" }
func (e exitCodeError) ExitCode() int { return int(e) }

// A run in a container ends with the signal that reached its command, the
// code its command exited with, or, its timeout expired and its commander
// ending with the error of its context and no code, killed: the run is
// recorded so, rather than left unrecorded. Another error is the error of
// the run.
func TestRunExitCode(t *testing.T) {
	live := context.Background()
	expired, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-expired.Done()

	code, err := runExitCode(live, syscall.SIGTERM, nil)
	require.NoError(t, err)
	assert.Equal(t, 128+int(syscall.SIGTERM), code, "a signal")

	code, err = runExitCode(live, nil, exitCodeError(3))
	require.NoError(t, err)
	assert.Equal(t, 3, code, "an exit code")

	code, err = runExitCode(expired, nil, errors.New("cancelled"))
	require.NoError(t, err)
	assert.Equal(t, 128+int(syscall.SIGKILL), code, "a timed out commander")

	code, err = runExitCode(expired, nil, exitCodeError(137))
	require.NoError(t, err)
	assert.Equal(t, 137, code, "a timed out process says its code")

	// A process the timeout of the run killed has no exit code, -1: the
	// run ended killed.
	short, cancelShort := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelShort()
	killedErr := exec.CommandContext(short, "sleep", "5").Run()
	var ee *exec.ExitError
	require.ErrorAs(t, killedErr, &ee)
	require.Equal(t, -1, ee.ExitCode(), "a killed process has no exit code")
	code, err = runExitCode(short, nil, killedErr)
	require.NoError(t, err)
	assert.Equal(t, 128+int(syscall.SIGKILL), code, "a process killed on timeout")

	_, err = runExitCode(live, nil, errors.New("agent unreachable"))
	assert.Error(t, err, "another error")

	code, err = runExitCode(live, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, code, "success")
}
