package tombmon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type pubSpy struct {
	msgs []pubsub.Messager
}

func (p *pubSpy) Pub(m pubsub.Messager, _ ...pubsub.Label) {
	p.msgs = append(p.msgs, m)
}

const id = "6e28098b-30bd-427c-941a-5b967353d0dd"

var version = time.Date(2026, 11, 17, 15, 33, 47, 0, time.UTC)

func setup(t *testing.T) *pubSpy {
	t.Helper()
	tombstone.InitData()
	node.InitData()
	instance.InitData()
	clusternode.Set([]string{"n1", "n2"})
	spy := &pubSpy{}
	mu.Lock()
	dir = filepath.Join(t.TempDir(), "tombstone")
	localhost = "n1"
	publisher = spy
	log = plog.NewDefaultLogger()
	started = true
	acks = make(map[string]map[string]time.Time)
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		started = false
		mu.Unlock()
		tombstone.InitData()
		node.InitData()
		instance.InitData()
	})
	return spy
}

func rec() tombstone.Record {
	return tombstone.Record{OrchestrationID: id, Path: "tombtest/svc/s1", Kind: tombstone.KindDeleted, Version: version}
}

// A record is written before Record returns, and published trusted.
func TestRecord(t *testing.T) {
	spy := setup(t)
	require.NoError(t, Record(rec()))
	l, err := tombstone.Load(dir)
	require.NoError(t, err)
	require.Len(t, l, 1)
	k := tombstone.Data.KilledBy("tombtest/svc/s1", version)
	require.Len(t, k, 1)
	assert.True(t, k[0].Trusted)
	require.Len(t, spy.msgs, 1)
	assert.IsType(t, &msgbus.NodeTombstoneUpdated{}, spy.msgs[0])

	mu.Lock()
	started = false
	mu.Unlock()
	assert.ErrorIs(t, Record(rec()), ErrNotStarted, "nothing deleted without its record")
}

// A record goes once kept Retention, every cluster node up, and no node
// publishing a config it kills.
func TestSweep(t *testing.T) {
	spy := setup(t)
	require.NoError(t, Record(rec()))
	old := time.Now().Add(-2 * Retention)
	require.NoError(t, os.Chtimes(filepath.Join(dir, id+".json"), old, old))
	p := naming.Path{Namespace: "tombtest", Kind: naming.KindSvc, Name: "s1"}

	sweep()
	assert.Len(t, tombstone.Data.GetByNode("n1"), 1, "the cluster nodes not known up")

	node.MonitorData.Set("n1", &node.Monitor{State: node.MonitorStateIdle})
	node.MonitorData.Set("n2", &node.Monitor{State: node.MonitorStateIdle})
	instance.ConfigData.Set(p, "n2", &instance.Config{Path: p, UpdatedAt: version})
	sweep()
	assert.Len(t, tombstone.Data.GetByNode("n1"), 1, "n2 publishes the version deleted")

	instance.ConfigData.Set(p, "n2", &instance.Config{Path: p, UpdatedAt: version.Add(time.Second)})
	sweep()
	assert.Empty(t, tombstone.Data.GetByNode("n1"), "a newer version concerns it no more")
	l, _ := tombstone.Load(dir)
	assert.Empty(t, l, "the file is removed")
	assert.IsType(t, &msgbus.NodeTombstoneDeleted{}, spy.msgs[len(spy.msgs)-1])
}

// A record written lately is kept, whatever else: a peer may hold the
// version deleted unpublished.
func TestSweepKeepsARecentRecord(t *testing.T) {
	setup(t)
	node.MonitorData.Set("n1", &node.Monitor{State: node.MonitorStateIdle})
	node.MonitorData.Set("n2", &node.Monitor{State: node.MonitorStateIdle})
	require.NoError(t, Record(rec()))
	sweep()
	assert.Len(t, tombstone.Data.GetByNode("n1"), 1)
}

// ack publishes the instance monitor of node, its delete done in the
// orchestration of the record, or in another state.
func ack(node string, state instance.MonitorState) {
	onInstanceMonitorUpdated(&msgbus.InstanceMonitorUpdated{Node: node, Value: instance.Monitor{
		OrchestrationID: uuid.MustParse(id), State: state}})
}

// A record goes at once when each node it lists was seen deleting its
// instance in the orchestration, every node up and no node publishing the
// version deleted; a delete seen before the record was written counts.
func TestEarlyRemoval(t *testing.T) {
	setup(t)
	clusternode.Set([]string{"n1", "n2", "n3"})
	for _, n := range []string{"n1", "n2", "n3"} {
		node.MonitorData.Set(n, &node.Monitor{State: node.MonitorStateIdle})
	}
	r := rec()
	r.Nodes = []string{"n2", "n3"}
	ack("n2", instance.MonitorStateDeleteSuccess)
	require.NoError(t, Record(r))
	ack("n3", instance.MonitorStateDeleteProgress)
	ack("n1", instance.MonitorStateDeleteSuccess)
	assert.Len(t, tombstone.Data.GetByNode("n1"), 1, "n3 not seen done, n1 is the local node")

	p := naming.Path{Namespace: "tombtest", Kind: naming.KindSvc, Name: "s1"}
	instance.ConfigData.Set(p, "n3", &instance.Config{Path: p, UpdatedAt: version})
	ack("n3", instance.MonitorStateDeleteSuccess)
	assert.Len(t, tombstone.Data.GetByNode("n1"), 1, "n3 publishes the version deleted still")

	instance.ConfigData.Unset(p, "n3")
	sweep()
	assert.Empty(t, tombstone.Data.GetByNode("n1"))
	l, _ := tombstone.Load(dir)
	assert.Empty(t, l)
}

// A record of nodes away waits Retention: an away node is never seen
// deleting, and the cluster is not up.
func TestNoEarlyRemovalWithANodeAway(t *testing.T) {
	setup(t)
	node.MonitorData.Set("n1", &node.Monitor{State: node.MonitorStateIdle})
	node.MonitorData.Set("n2", &node.Monitor{State: node.MonitorStateMaintenance})
	r := rec()
	r.Nodes = []string{"n2"}
	require.NoError(t, Record(r))
	ack("n2", instance.MonitorStateDeleteSuccess)
	assert.Len(t, tombstone.Data.GetByNode("n1"), 1)
}
