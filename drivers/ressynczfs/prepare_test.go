package ressynczfs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
)

// fakeObject is the object of a resource under test: its var directory.
type fakeObject struct {
	varDir string
}

func (o fakeObject) Log() *plog.Logger                   { return plog.NewDefaultLogger() }
func (o fakeObject) VarDir() string                      { return o.varDir }
func (o fakeObject) ResourceByID(string) resource.Driver { return nil }
func (o fakeObject) ResourcesByDrivergroups([]driver.Group) resource.Drivers {
	return nil
}

// prepareContext is the context of an update of a node holding the started
// instance: the final sync of a switch is not asked where the instance runs,
// which takes the status of the resources of an object to tell.
func prepareContext() context.Context {
	return actioncontext.WithFinalSync(context.Background(), true)
}

// newPreparingT is a resource syncing to the peer n2, its var directory in
// the one of the test.
func newPreparingT(t *testing.T) (*T, *fakeCluster) {
	t.Helper()
	d := newTestT(t)
	d.SetObject(fakeObject{varDir: t.TempDir()})
	d.Nodes = []string{hostname.Hostname(), "n2"}
	d.Target = []string{"nodes"}
	return d, newFakeCluster(d)
}

// TestWillPrepareSync pins which runs take their snapshot with the ones of
// the other sync resources of the action: the ones sending something, unless
// delay_snap has the resource take it just before its own transfer.
func TestWillPrepareSync(t *testing.T) {
	ctx := prepareContext()
	d, c := newPreparingT(t)
	assert.True(t, d.CopiesSyncSnapshot())
	assert.True(t, d.WillPrepareSync(ctx))

	d.DelaySnap = true
	assert.False(t, d.CopiesSyncSnapshot())
	assert.False(t, d.WillPrepareSync(ctx), "delay_snap")
	d.DelaySnap = false

	d.Target = nil
	assert.False(t, d.WillPrepareSync(ctx), "no peer to send to")
	d.Target = []string{"nodes"}

	assert.False(t, d.WillPrepareSync(actioncontext.WithTarget(ctx, []string{"drpnodes"})), "no peer of the target asked")

	d.ObjectEnv, d.NodeEnv = "PRD", "DEV"
	assert.False(t, d.WillPrepareSync(ctx), "a PRD object on a node that is not")
	d.ObjectEnv, d.NodeEnv = "", ""

	d.Topology = topology.Flex
	d.Nodes = []string{"n2", hostname.Hostname()}
	assert.False(t, d.WillPrepareSync(ctx), "a flex instance not primary")

	assert.Empty(t, c.snaps[""], "asking takes no snapshot")
}

// TestPrepareSyncTakesTheSnapshotTheRunSends pins that the run of the action
// sends the snapshot PrepareSync took, taking no other, and that the release
// keeps it, as the base of the next run.
func TestPrepareSyncTakesTheSnapshotTheRunSends(t *testing.T) {
	ctx := prepareContext()
	d, c := newPreparingT(t)

	release, err := d.PrepareSync(ctx)
	require.NoError(t, err)
	require.NotNil(t, release)
	require.Len(t, c.snaps[""], 1)
	taken := c.snaps[""][0].Name
	assert.True(t, d.isOwnSnapshot(taken))

	name, now, err := d.runSnapshot()
	require.NoError(t, err)
	assert.Equal(t, taken, name, "the run sends the snapshot of the action")
	assert.Equal(t, d.newSnapName(now), name, "at the time it was taken")
	assert.Len(t, c.snaps[""], 1, "and takes no other")

	// The run found it the newest, and sends it.
	d.prepared.sent = true
	require.NoError(t, release())
	assert.Nil(t, d.prepared)
	assert.Len(t, c.snaps[""], 1, "the snapshot sent is the base of the next run")

	name, _, err = d.runSnapshot()
	require.NoError(t, err)
	assert.NotEqual(t, taken, name, "released, the next run takes its own")
	assert.Len(t, c.snaps[""], 2)
}

// TestPrepareSyncReleaseDestroysASnapshotNotSent pins that the snapshot of
// an action failing before the transfer of the resource is destroyed: left
// the newest, the next run would take it for one received from another
// source, and forget what this node knows of its peers. The snapshots of the
// former runs are kept.
func TestPrepareSyncReleaseDestroysASnapshotNotSent(t *testing.T) {
	ctx := prepareContext()
	d, c := newPreparingT(t)
	require.NoError(t, c.takeSnapshot(d.newSnapName(t0)))

	release, err := d.PrepareSync(ctx)
	require.NoError(t, err)
	require.Len(t, c.snaps[""], 2)
	require.NoError(t, release())
	assert.Equal(t, []string{snapAt(0)}, c.names(""))
}

// TestPrepareSyncTakesNothingWithNoPeer pins that a resource
// sending nothing takes no snapshot, and holds nothing.
func TestPrepareSyncTakesNothingWithNoPeer(t *testing.T) {
	ctx := prepareContext()
	d, c := newPreparingT(t)
	d.Target = nil
	release, err := d.PrepareSync(ctx)
	require.NoError(t, err)
	assert.Nil(t, release)
	assert.Nil(t, d.prepared)
	assert.Empty(t, c.snaps[""])
}
