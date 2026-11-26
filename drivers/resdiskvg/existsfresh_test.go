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

func TestExistsFreshRescanFailing(t *testing.T) {
	vg := &fakeStaleVG{before: true, after: true}
	var logged []string
	debugf := func(format string, args ...any) { logged = append(logged, format) }
	exists, err := existsFreshOf(context.Background(), vg, func() error { return errors.New("pvscan failed") }, debugf)
	require.NoError(t, err)
	assert.True(t, exists, "the vg is checked still")
	assert.Len(t, logged, 1)
}

func TestExistsFreshError(t *testing.T) {
	vg := &fakeStaleVG{err: errors.New("vgs failed")}
	_, err := existsFreshOf(context.Background(), vg, vg.rescan, noDebug)
	require.ErrorContains(t, err, "vgs failed")
}
