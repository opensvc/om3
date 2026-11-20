package ressynczfs

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
)

// prepareContext is the context of an update of a node holding the started
// instance: the final sync of a switch is not asked where the instance runs,
// which takes the status of the resources of an object to tell.
func prepareContext() context.Context {
	return actioncontext.WithFinalSync(context.Background(), true)
}

// TestPrepareSyncTakesTheSnapshots pins that the snapshots of the datasets
// are taken before the first transfer of the action, as v2 took them, the
// update of the action then taking none, and that the release has the next
// update take its own.
func TestPrepareSyncTakesTheSnapshots(t *testing.T) {
	ctx := prepareContext()
	var taken []string
	d := &T{Dataset: []string{"pool/a", "pool/b"}, Name: "daily"}
	d.takeSnap = func(name string) error {
		taken = append(taken, name)
		return nil
	}
	assert.True(t, d.WillPrepareSync(ctx))
	assert.False(t, (&T{}).WillPrepareSync(ctx), "no dataset")

	release, err := d.PrepareSync(ctx)
	require.NoError(t, err)
	require.NotNil(t, release)
	require.Len(t, taken, 2)
	assert.True(t, strings.HasPrefix(taken[0], "pool/a@daily.snap."), taken[0])
	assert.True(t, strings.HasPrefix(taken[1], "pool/b@daily.snap."), taken[1])
	assert.Equal(t, map[string]bool{"pool/a": true, "pool/b": true}, d.prepared, "the update takes none of them again")

	require.NoError(t, release())
	assert.Nil(t, d.prepared, "the next update takes its own")
}

// TestPrepareSyncRemembersTheSnapshotsTakenBeforeAFailure pins that the
// update of an optional resource, run after its PrepareSync failed, takes
// the snapshots it misses only.
func TestPrepareSyncRemembersTheSnapshotsTakenBeforeAFailure(t *testing.T) {
	d := &T{Dataset: []string{"pool/a", "pool/b"}}
	d.takeSnap = func(name string) error {
		if strings.HasPrefix(name, "pool/b@") {
			return errors.New("no such dataset")
		}
		return nil
	}
	release, err := d.PrepareSync(prepareContext())
	assert.ErrorContains(t, err, "no such dataset")
	assert.Nil(t, release)
	assert.Equal(t, map[string]bool{"pool/a": true}, d.prepared)
}
