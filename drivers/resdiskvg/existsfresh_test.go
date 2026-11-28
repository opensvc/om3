//go:build linux

package resdiskvg

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStaleVG is a volume group the lvm cache lists as before until it is
// rescanned.
type fakeStaleVG struct {
	VGDriver
	before, after bool
	err           error
	rescanned     bool
}

func (t *fakeStaleVG) Exists(context.Context) (bool, error) {
	if t.err != nil {
		return false, t.err
	}
	if t.rescanned {
		return t.after, nil
	}
	return t.before, nil
}

func (t *fakeStaleVG) rescan() error {
	t.rescanned = true
	return nil
}

func noDebug(string, ...any) {}

// TestExistsFresh checks a provision decides on the volume group as the lvm
// cache lists it once refreshed, whatever it listed before.
func TestExistsFresh(t *testing.T) {
	ctx := context.Background()

	// A vg on a loop disk the same provision attached: the cache taken
	// before missed it, the provision must not create it again.
	vg := &fakeStaleVG{before: false, after: true}
	exists, err := existsFreshOf(ctx, vg, vg.rescan, noDebug)
	require.NoError(t, err)
	assert.True(t, exists)

	// A vg of a shared storage another node removed: the cache of this
	// node still lists it.
	vg = &fakeStaleVG{before: true, after: false}
	exists, err = existsFreshOf(ctx, vg, vg.rescan, noDebug)
	require.NoError(t, err)
	assert.False(t, exists)
}

// A rescan failing drops the cache still: the vg exists when listed before or
// after, a vg the cache listed not taken for absent on the word of a scan
// failing, which would have the provision create it again.
func TestExistsFreshRescanFailing(t *testing.T) {
	for _, c := range []struct {
		before, after, want bool
	}{
		{true, true, true},
		{true, false, true},
		{false, true, true},
		{false, false, false},
	} {
		vg := &fakeStaleVG{before: c.before, after: c.after}
		var logged []string
		debugf := func(format string, args ...any) { logged = append(logged, format) }
		failingRescan := func() error {
			vg.rescanned = true // the cache dropped anyway
			return errors.New("pvscan failed")
		}
		exists, err := existsFreshOf(context.Background(), vg, failingRescan, debugf)
		require.NoError(t, err)
		assert.Equal(t, c.want, exists, "before %v, after %v", c.before, c.after)
		assert.Len(t, logged, 1)
	}
}

// A provision cancelled during the rescan makes no decision.
func TestExistsFreshCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	vg := &fakeStaleVG{before: false, after: false}
	rescan := func() error {
		cancel()
		return context.Canceled
	}
	_, err := existsFreshOf(ctx, vg, rescan, noDebug)
	require.ErrorIs(t, err, context.Canceled)
}

func TestExistsFreshError(t *testing.T) {
	vg := &fakeStaleVG{err: errors.New("vgs failed")}
	_, err := existsFreshOf(context.Background(), vg, vg.rescan, noDebug)
	require.ErrorContains(t, err, "vgs failed")
}
