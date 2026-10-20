package rescontainerocibase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/args"
)

// quietArgs is the argser of a task container, whose run output is read
// from its logs after the run: the logs command is a shell script here.
type quietArgs struct {
	ExecutorArgser
	logsScript string
}

func (a quietArgs) QuietRunOutput() bool {
	return true
}

func (a quietArgs) LogsSinceArgs(time.Time) *args.T {
	return args.New("-c", a.logsScript)
}

// A command whose output is not logged as it comes keeps what it says of a
// failure for the error: an engine failing to run a task container says why.
func TestQuietRunKeepsTheFailureReason(t *testing.T) {
	e := NewExecutor("/bin/sh", quietArgs{}, testLogger{})
	assert.True(t, e.quietRun())
	err := e.doExecRunLog(context.Background(), false, nil, "-c", "echo out; echo first >&2; echo 'Error: no such image' >&2; exit 125")
	require.Error(t, err)
	assert.ErrorContains(t, err, "Error: no such image")
	assert.NoError(t, e.doExecRunLog(context.Background(), false, nil, "-c", "echo quiet; echo quiet >&2"))
}

// The logs of a container read since a time come as they were written, the
// stdout lines apart from the stderr ones.
func TestLogsSinceSplitsTheStreams(t *testing.T) {
	e := NewExecutor("/bin/sh", quietArgs{logsScript: "echo o1; echo e1 >&2; echo o2"}, testLogger{})
	var stdout, stderr []string
	err := e.LogsSince(context.Background(), time.Now(),
		func(s string) { stdout = append(stdout, s) },
		func(s string) { stderr = append(stderr, s) },
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"o1", "o2"}, stdout)
	assert.Equal(t, []string{"e1"}, stderr)
}
