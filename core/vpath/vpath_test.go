package vpath

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/status"
)

// A vol on standby disks, as one of the drbd pool, is standby up with up once
// its filesystem is mounted: its paths are there, as the ones of a vol up.
// With its filesystem not mounted, they are not.
func TestAccessible(t *testing.T) {
	for avail, want := range map[status.T]bool{
		status.Up:                true,
		status.NotApplicable:     true,
		status.StandbyUp:         true,
		status.StandbyUpWithUp:   true,
		status.StandbyUpWithDown: false,
		status.StandbyDown:       false,
		status.Down:              false,
		status.Warn:              false,
		status.Undef:             false,
	} {
		assert.Equal(t, want, accessible(avail), avail.String())
	}
}

// The error says the standby states apart, rather than the string of up a
// standby up with up has, which would contradict it.
func TestErrAccessSaysTheStandbyStatesApart(t *testing.T) {
	assert.Equal(t, "resource is not accessible: volume#0 is avail stdby up with its other resources down",
		ErrAccess{RID: "volume#0", Avail: status.StandbyUpWithDown}.Error())
	assert.Equal(t, "vol is not accessible: test/vol/data is avail down",
		ErrAccess{Path: naming.Path{Namespace: "test", Kind: naming.KindVol, Name: "data"}, Avail: status.Down}.Error())
}

// A volume resource of a vol on the drbd pool is standby up with up once
// started: a container mounts a path in it, and refuses one in it while its
// filesystem is not mounted.
func TestResolveAVolumeOnStandbyDisks(t *testing.T) {
	ctx := context.Background()
	o := fakeObject{
		"volume#0": fakeResource{head: "/srv/vm-data.test.vol.c1", avail: status.StandbyUpWithUp},
		"volume#1": fakeResource{head: "/srv/vm-cfg.test.vol.c1", avail: status.StandbyUpWithDown},
	}
	got, err := Resolve(ctx, "volume#0:/d", "test", o)
	require.NoError(t, err)
	assert.Equal(t, "/srv/vm-data.test.vol.c1/d", got.HostPath)

	_, err = Resolve(ctx, "volume#1:/d", "test", o)
	var errAccess ErrAccess
	require.True(t, errors.As(err, &errAccess), "the filesystem is not mounted: %v", err)
	assert.ErrorContains(t, err, "stdby up with its other resources down")
}
