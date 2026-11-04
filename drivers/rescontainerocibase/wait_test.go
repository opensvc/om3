package rescontainerocibase

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/util/args"
)

// waitArgs is the argser of an engine whose wait fails, as podman 1.6 on
// options it does not know. Its inspect is a shell script: failing, the
// container is gone; else it prints running or stopped.
type waitArgs struct {
	ExecutorArgser
	inspectScript string
}

type waitInspect struct {
	Inspecter
	running bool
}

var errWaitUnknownFlag = errors.New("unknown flag: --condition")

func (a waitArgs) Credential() (*Credential, error) {
	return nil, nil
}

func (a waitArgs) WaitNotRunning(context.Context) error {
	return errWaitUnknownFlag
}

func (a waitArgs) WaitRemoved(context.Context) error {
	return errWaitUnknownFlag
}

func (a waitArgs) InspectArgs() *args.T {
	return args.New("-c", a.inspectScript)
}

func (a waitArgs) InspectParser(b []byte) (Inspecter, error) {
	return waitInspect{running: string(b) == "running\n"}, nil
}

func (i waitInspect) Running() bool {
	return i.running
}

func newWaitExecutor(script string) *Executor {
	return NewExecutor("/bin/sh", waitArgs{inspectScript: script}, testLogger{})
}

// A wait the engine fails is what the wait was for when the container is
// not running, or gone: the stop did stop it.
func TestAFailedWaitOfAContainerNotRunningIsNotAnError(t *testing.T) {
	assert.NoError(t, newWaitExecutor("echo stopped").WaitNotRunning(context.Background()))
	assert.NoError(t, newWaitExecutor("exit 1").WaitNotRunning(context.Background()))
}

// A wait the engine fails while the container runs is the error it is.
func TestAFailedWaitOfAContainerRunningIsAnError(t *testing.T) {
	err := newWaitExecutor("echo running").WaitNotRunning(context.Background())
	assert.ErrorIs(t, err, errWaitUnknownFlag)
}

// A wait for the removal the engine fails is judged by inspect for a while:
// the engine may remove an auto-removed container a moment after its stop.
func TestAFailedWaitOfARemovalWaitsForTheContainerToBeGone(t *testing.T) {
	defer func(timeout, interval time.Duration) {
		waitRemovedByInspectTimeout, waitRemovedByInspectInterval = timeout, interval
	}(waitRemovedByInspectTimeout, waitRemovedByInspectInterval)
	waitRemovedByInspectTimeout, waitRemovedByInspectInterval = 5*time.Second, 10*time.Millisecond

	// Gone at the third inspect.
	counter := filepath.Join(t.TempDir(), "inspects")
	script := fmt.Sprintf("echo x >> %[1]s; [ $(wc -l < %[1]s) -lt 3 ] && echo stopped || exit 1", counter)
	assert.NoError(t, newWaitExecutor(script).WaitRemoved(context.Background()))

	waitRemovedByInspectTimeout = 100 * time.Millisecond
	err := newWaitExecutor("echo stopped").WaitRemoved(context.Background())
	assert.ErrorIs(t, err, errWaitUnknownFlag)
}
