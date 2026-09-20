package rescontainerocibase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
)

// deadlineExecuter is an executer whose pull and run are ended by the
// deadline set on them, which is what a container runtime that never answers
// looks like from here.
type deadlineExecuter struct {
	Executer
	hasImage bool
}

func (e *deadlineExecuter) HasImage(context.Context) (bool, string, error) {
	return e.hasImage, "", nil
}

func (e *deadlineExecuter) Pull(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (e *deadlineExecuter) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (e *deadlineExecuter) InspectRefresh(context.Context) (Inspecter, error) {
	return nil, nil
}

func btWithTimeouts(t *testing.T, pull, start time.Duration, hasImage bool) *BT {
	t.Helper()
	bt := &BT{
		T:            resource.T{ResourceID: &resourceid.T{Name: "container#1"}},
		Path:         naming.Path{Name: "foo", Kind: naming.KindSvc},
		Image:        "ghcr.io/opensvc/foo:1",
		PullTimeout:  &pull,
		StartTimeout: &start,
	}
	require.NoError(t, bt.Configure())
	bt.executer = &deadlineExecuter{hasImage: hasImage}
	return bt
}

// A deadline reads as "context deadline exceeded" wherever it surfaces, and
// says neither which keyword set it nor what it was set to. Whoever reads the
// error has to know which of the two windows ended, or the wrong keyword gets
// raised.
func TestATimedOutStartNamesTheTimeoutThatEndedIt(t *testing.T) {
	t.Run("the image is already local, so no pull is in the window", func(t *testing.T) {
		bt := btWithTimeouts(t, time.Minute, 10*time.Millisecond, true)
		err := bt.pullAndRun(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "did not start within start_timeout (10ms)")
		assert.Contains(t, err.Error(), "already local")
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("the image was pulled first, under its own timeout", func(t *testing.T) {
		bt := btWithTimeouts(t, 10*time.Millisecond, time.Minute, false)
		err := bt.pullAndRun(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "was not pulled within pull_timeout (10ms)",
			"a pull that runs out of time says so, and names its own keyword")
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
}
