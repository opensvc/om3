package object_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/testhelper"

	_ "github.com/opensvc/om3/v3/drivers/ressyncbtrfs"
	_ "github.com/opensvc/om3/v3/drivers/ressyncbtrfssnap"
	_ "github.com/opensvc/om3/v3/drivers/ressyncrsync"
	_ "github.com/opensvc/om3/v3/drivers/ressynczfs"
	_ "github.com/opensvc/om3/v3/drivers/ressynczfssnap"
)

// snapSyncsConfig has a sync resource of each driver taking snapshots, and
// of each way out of the grouping of their snapshots.
const snapSyncsConfig = `[sync#1]
type = rsync
src = /srv/a/
dst = /srv/a/
target = nodes
snap = true
schedule = @1h

[sync#2]
type = zfs
src = pool/a
dst = pool/a
target = nodes
schedule = @1h

[sync#3]
type = btrfs
src = fs1:data
target = nodes
schedule = @1d

[sync#4]
type = rsync
src = /srv/b/
dst = /srv/b/
target = nodes
snap = true
delay_snap = true
schedule = @1d

[sync#5]
type = rsync
src = /srv/c/
dst = /srv/c/
target = nodes
schedule = @1d

[sync#6]
type = zfs
src = pool/b
dst = pool/b
target = nodes
schedule = @1d

[sync#7]
type = zfs
src = pool/c
dst = pool/c
target = nodes
delay_snap = true
schedule = @2d

[sync#8]
type = btrfs
src = fs1:logs
target = nodes
delay_snap = true
schedule = @2d

[sync#9]
type = zfssnap
dataset = pool/a
schedule = @2d

[sync#10]
type = btrfssnap
subvol = fs1:data
schedule = @3d
`

func newSnapSyncsSvc(t *testing.T) object.Svc {
	t.Helper()
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)
	p, err := naming.ParsePath("test/svc/snapsched")
	require.NoError(t, err)
	o, err := object.NewSvc(p, object.WithConfigData([]byte(snapSyncsConfig)), object.WithVolatile(true))
	require.NoError(t, err)
	return o
}

// The syncs of an object copying a snapshot, a sync.rsync with snap, a
// sync.zfs or a sync.btrfs, whose schedule differs from the first's are
// warned of: the scheduler runs together the syncs due at once only, so
// their snapshots are taken in separate runs. A sync with delay_snap, or
// with no snap, is not grouped, and not warned of. Nor is a sync.zfssnap or
// a sync.btrfssnap, whose snapshots are kept on a schedule of their own.
func TestSnapScheduleAlerts(t *testing.T) {
	o := newSnapSyncsSvc(t)
	all, err := o.ValidateConfig(context.Background())
	require.NoError(t, err)
	alerts := make(xconfig.Alerts, 0)
	for _, a := range all {
		if strings.Contains(a.Comment, "taken in separate runs") {
			alerts = append(alerts, a)
		}
	}
	require.Len(t, alerts, 2, "%v", alerts)
	assert.Equal(t, "sync#3.schedule", alerts[0].Key.String())
	assert.Equal(t, "sync.btrfs", alerts[0].Driver.String())
	assert.Contains(t, alerts[0].Comment, "differs from sync#1.schedule")
	assert.Contains(t, alerts[0].Comment, "delay_snap = true")
	assert.Equal(t, "sync#6.schedule", alerts[1].Key.String())
	assert.Equal(t, "sync.zfs", alerts[1].Driver.String())
}

// The sync drivers taking snapshots all take them before the first transfer
// of an action, as resource.SyncPreparer, and the ones copying theirs say,
// from their configuration, whether they take it with the others.
func TestTheSnapshottingSyncDriversPrepare(t *testing.T) {
	o := newSnapSyncsSvc(t)
	copies := make(map[string]bool)
	lister, ok := any(o).(interface {
		ResourcesByDrivergroups([]driver.Group) resource.Drivers
	})
	require.True(t, ok)
	for _, r := range lister.ResourcesByDrivergroups([]driver.Group{driver.GroupSync}) {
		require.NoError(t, r.GetConfigurationError(), r.RID())
		_, ok := any(r).(resource.SyncPreparer)
		assert.True(t, ok, "%s (%s) takes its snapshot before the first transfer", r.RID(), r.DriverID())
		copies[r.RID()] = resource.CopiesSyncSnapshot(r)
	}
	assert.Equal(t, map[string]bool{
		"sync#1":  true,
		"sync#2":  true,
		"sync#3":  true,
		"sync#4":  false,
		"sync#5":  false,
		"sync#6":  true,
		"sync#7":  false,
		"sync#8":  false,
		"sync#9":  false,
		"sync#10": false,
	}, copies)
}

// An encap sync never shares a run with the syncs of its host: on a node
// not of the encap nodes, it is not compared with them, whatever their
// schedules, and the syncs of the host still are among themselves.
func TestSnapScheduleAlertsLeaveTheEncapSyncsOut(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)
	p, err := naming.ParsePath("test/svc/snapencap")
	require.NoError(t, err)
	o, err := object.NewSvc(p, object.WithConfigData([]byte(`[DEFAULT]
encapnodes = encap1

[sync#1]
type = zfs
src = pool/e
dst = pool/e
target = nodes
encap = true
schedule = @1d

[sync#2]
type = zfs
src = pool/a
dst = pool/a
target = nodes
schedule = @1h

[sync#3]
type = zfs
src = pool/b
dst = pool/b
target = nodes
schedule = @1h

[sync#4]
type = zfs
src = pool/c
dst = pool/c
target = nodes
schedule = @2h
`)), object.WithVolatile(true))
	require.NoError(t, err)
	all, err := o.ValidateConfig(context.Background())
	require.NoError(t, err)
	var keys []string
	for _, a := range all {
		if strings.Contains(a.Comment, "taken in separate runs") {
			keys = append(keys, a.Key.String()+" "+a.Comment[:strings.Index(a.Comment, ":")])
		}
	}
	assert.Equal(t, []string{"sync#4.schedule differs from sync#2.schedule"}, keys)
}
