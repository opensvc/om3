package ressyncbtrfs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
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

// newPreparingT is a resource syncing to the peer b, its var directory in
// the one of the test, its source subvolume holding v1.
func newPreparingT(t *testing.T) (*T, *fakeCluster) {
	t.Helper()
	o := newTestT(t)
	o.SetObject(fakeObject{varDir: t.TempDir()})
	o.Nodes = []string{hostname.Hostname(), "b"}
	c := newFakeCluster(o)
	c.write("", "v1")
	return o, c
}

// TestWillPrepareSync pins which syncs take their run with the snapshots of
// the other sync resources of the action: the ones sending something, unless
// delay_snap has the resource take it just before its own transfer.
func TestWillPrepareSync(t *testing.T) {
	ctx := prepareContext()
	o, c := newPreparingT(t)
	assert.True(t, o.CopiesSyncSnapshot())
	assert.True(t, o.WillPrepareSync(ctx))

	o.DelaySnap = true
	assert.False(t, o.CopiesSyncSnapshot())
	assert.False(t, o.WillPrepareSync(ctx), "delay_snap")
	o.DelaySnap = false

	assert.False(t, o.WillPrepareSync(actioncontext.WithTarget(ctx, []string{"drpnodes"})), "no peer of the target asked")

	o.Target = []string{"local"}
	assert.False(t, o.WillPrepareSync(ctx), "a sync of the subvolume to itself is refused")
	o.Target = []string{"nodes"}

	o.Src = ".osync/x"
	assert.False(t, o.WillPrepareSync(ctx), "a src that is no <label>:<subvol>")
	o.Src = "fs1:data"

	o.ObjectEnv, o.NodeEnv = "PRD", "DEV"
	assert.False(t, o.WillPrepareSync(ctx), "a PRD object on a node that is not")

	assert.Empty(t, c.runNames(""), "asking takes no run")
}

// TestPrepareSyncTakesTheRunTheSyncSends pins that the sync of the action
// sends the run PrepareSync took, nested subvolumes included, taking no
// other, and that the release keeps it, as the base of the next sync.
func TestPrepareSyncTakesTheRunTheSyncSends(t *testing.T) {
	ctx := prepareContext()
	o, c := newPreparingT(t)
	o.Recursive = true
	c.node("")["data/nested"] = c.newSubvol("n1", false, "")

	release, err := o.PrepareSync(ctx)
	require.NoError(t, err)
	require.NotNil(t, release)
	names := c.runNames("")
	require.Len(t, names, 1)
	assert.Equal(t, []string{"", "nested"}, o.prepared.run.rels(), "the nested subvolumes are in the run")

	// Written after the snapshots of the action: not in what is sent.
	c.write("", "v2")

	r, previous, now, err := o.syncRun()
	require.NoError(t, err)
	assert.Equal(t, names[0], r.Name, "the sync sends the run of the action")
	assert.Equal(t, newRunName(now), r.Name, "at the time it was taken")
	assert.Empty(t, previous, "which is not one of the runs held before it")
	assert.Equal(t, names, c.runNames(""), "and takes no other")
	head, _ := r.head()
	assert.Equal(t, "v1", c.node("")[head.Path].content)

	require.NoError(t, release())
	assert.Nil(t, o.prepared)
	assert.Equal(t, names, c.runNames(""), "the run sent is the base of the next sync")

	time.Sleep(time.Millisecond)
	r, previous, _, err = o.syncRun()
	require.NoError(t, err)
	assert.NotEqual(t, names[0], r.Name, "released, the next sync takes its own")
	require.Len(t, previous, 1)
	assert.Equal(t, names[0], previous[0].Name)
}

// TestPrepareSyncReleaseDeletesARunNotSent pins that the run of an action
// failing before the transfer of the resource is deleted: left the newest,
// the next sync would take it for one received from another source, and
// forget what this node knows of its peers. The former runs are kept.
func TestPrepareSyncReleaseDeletesARunNotSent(t *testing.T) {
	ctx := prepareContext()
	o, c := newPreparingT(t)
	t0 := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	require.NoError(t, c.sync(modeIncr, t0, "b"))
	require.Equal(t, []string{newRunName(t0)}, c.runNames(""))

	release, err := o.PrepareSync(ctx)
	require.NoError(t, err)
	require.Len(t, c.runNames(""), 2)
	require.NoError(t, release())
	assert.Equal(t, []string{newRunName(t0)}, c.runNames(""))
}

// TestPrepareSyncTakesNothingWithNoPeer pins that a resource sending nothing
// takes no run, and holds nothing.
func TestPrepareSyncTakesNothingWithNoPeer(t *testing.T) {
	o, c := newPreparingT(t)
	release, err := o.PrepareSync(actioncontext.WithTarget(prepareContext(), []string{"drpnodes"}))
	require.NoError(t, err)
	assert.Nil(t, release)
	assert.Nil(t, o.prepared)
	assert.Empty(t, c.runNames(""))
}
