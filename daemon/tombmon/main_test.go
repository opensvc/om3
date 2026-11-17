package tombmon

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clusternode"
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

func newTestState(t *testing.T) (*state, *pubSpy) {
	t.Helper()
	tombstone.InitData()
	node.InitData()
	clusternode.Set([]string{"n1", "n2", "n3"})
	spy := &pubSpy{}
	s := &state{
		dir:       filepath.Join(t.TempDir(), "tombstone"),
		localhost: "n1",
		publisher: spy,
		log:       plog.NewDefaultLogger(),
		records:   make(map[string]tombstone.Record),
		baseline:  make(map[string]time.Time),
		byPath:    make(map[string]map[string]bool),
		dirty:     make(map[string]bool),
		removed:   make(map[string]bool),
		nodeAway:  make(map[string]bool),
		gone:      make(map[string]goneRecord),
	}
	mu.Lock()
	st = s
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		st = nil
		mu.Unlock()
		tombstone.InitData()
		node.InitData()
	})
	return s, spy
}

func record(holders map[string]tombstone.Holder) tombstone.Record {
	return tombstone.Record{OrchestrationID: id, Path: "system/sec/s1", Kind: tombstone.KindDeleted, Holders: holders}
}

func pending(mode tombstone.HolderMode) tombstone.Holder {
	return tombstone.Holder{State: tombstone.HolderPending, Mode: mode}
}

func allUp(nodes ...string) {
	for _, n := range nodes {
		node.MonitorData.Set(n, &node.Monitor{State: node.MonitorStateIdle})
	}
}

// A record is persisted before it is published, held as the one of the
// node, and announced.
func TestRecordPersistsThenPublishes(t *testing.T) {
	s, spy := newTestState(t)
	require.NoError(t, Record(record(map[string]tombstone.Holder{"n2": pending(tombstone.HolderLive)})))
	l, err := tombstone.Load(s.dir)
	require.NoError(t, err)
	require.Len(t, l, 1)
	_, ok := tombstone.Data.Get("n1", id)
	assert.True(t, ok)
	require.Len(t, spy.msgs, 1, "published as Record returns")
	assert.IsType(t, &msgbus.NodeTombstoneUpdated{}, spy.msgs[0])

	require.NoError(t, Record(record(map[string]tombstone.Holder{"n2": pending(tombstone.HolderLive)})))
	assert.Len(t, spy.msgs, 1, "the same record again changes nothing")
}

// A peer copy is adopted only with a pending holder; adopted before the
// rejoin of the node, its own live holder turns away.
func TestOnPeerRecord(t *testing.T) {
	s, _ := newTestState(t)
	s.onPeerRecord(record(map[string]tombstone.Holder{"n2": {State: tombstone.HolderCleared, ClearedBy: tombstone.ClearedByDeleted}}))
	assert.Empty(t, s.records, "an all-cleared copy is not adopted")

	s.onPeerRecord(record(map[string]tombstone.Holder{"n1": pending(tombstone.HolderLive)}))
	require.Contains(t, s.records, id)
	assert.Equal(t, tombstone.HolderAway, s.records[id].Holders["n1"].Mode, "adopted before the rejoin")

	delete(s.records, id)
	node.MonitorData.Set("n1", &node.Monitor{State: node.MonitorStateIdle})
	s.onPeerRecord(record(map[string]tombstone.Holder{"n1": pending(tombstone.HolderLive)}))
	assert.Equal(t, tombstone.HolderLive, s.records[id].Holders["n1"].Mode, "adopted once rejoined")
}

// A live holder whose node goes away turns away; an evicted holder is
// cleared.
func TestDemoteAndEvict(t *testing.T) {
	s, _ := newTestState(t)
	require.NoError(t, Record(record(map[string]tombstone.Holder{"n2": pending(tombstone.HolderLive), "n4": pending(tombstone.HolderAway)})))
	s.handle(&msgbus.NodeMonitorUpdated{Node: "n2", Value: node.Monitor{State: node.MonitorStateMaintenance}})
	assert.Equal(t, tombstone.HolderAway, s.records[id].Holders["n2"].Mode)
	s.handle(&msgbus.ClusterConfigUpdated{})
	h := s.records[id].Holders["n4"]
	assert.Equal(t, tombstone.HolderCleared, h.State, "n4 is no cluster node")
	assert.Equal(t, tombstone.ClearedByEvicted, h.ClearedBy)
}

// A record all cleared is removed once every node is up and no node
// publishes it with a pending holder; a node in maintenance holds it.
func TestRemoval(t *testing.T) {
	s, spy := newTestState(t)
	// n2, the local node, is the holder: as a live holder clears itself
	// once its instance is deleted.
	s.localhost = "n2"
	require.NoError(t, Record(record(map[string]tombstone.Holder{"n2": pending(tombstone.HolderLive)})))
	allUp("n1", "n2")
	node.MonitorData.Set("n3", &node.Monitor{State: node.MonitorStateMaintenance})
	tombstone.Data.Set("n3", record(map[string]tombstone.Holder{"n2": pending(tombstone.HolderLive)}))

	require.NoError(t, ClearLocal("system/sec/other", tombstone.ClearedByDeleted))
	assert.True(t, s.records[id].IsPending("n2"), "no record of this path")

	require.NoError(t, ClearLocal("system/sec/s1", tombstone.ClearedByDeleted))
	assert.False(t, s.records[id].IsPending("n2"))
	assert.Contains(t, s.records, id, "n3 is in maintenance")

	allUp("n3")
	s.removeAllIfDone()
	assert.Contains(t, s.records, id, "n3 still publishes a pending copy")

	tombstone.Data.Set("n3", s.records[id])
	s.handle(&msgbus.NodeTombstoneUpdated{Node: "n3", ID: id})
	assert.NotContains(t, s.records, id)
	flush()
	l, _ := tombstone.Load(s.dir)
	assert.Empty(t, l, "the file is removed")
	assert.IsType(t, &msgbus.NodeTombstoneDeleted{}, spy.msgs[len(spy.msgs)-1])

	s.onPeerRecord(record(map[string]tombstone.Holder{"n2": {State: tombstone.HolderCleared, ClearedBy: tombstone.ClearedByDeleted}}))
	assert.NotContains(t, s.records, id, "a stale all-cleared copy does not bring it back")

	s.onPeerRecord(record(map[string]tombstone.Holder{"n1": pending(tombstone.HolderLive)}))
	assert.NotContains(t, s.records, id, "nor does a stale copy naming other holders")
}

// A peer publishing again a record the node removed, as one killed before
// it wrote the last merge, naming the node pending, is answered: the node
// alone knows its holder was cleared, and nothing else would clear it. An
// explicit clear or drop of such a copy clears it too.
func TestStaleCopyNamingTheNode(t *testing.T) {
	s, _ := newTestState(t)
	s.localhost = "n2"
	allUp("n1", "n2", "n3")
	require.NoError(t, Record(record(map[string]tombstone.Holder{"n2": pending(tombstone.HolderLive)})))
	require.NoError(t, ClearLocal("system/sec/s1", tombstone.ClearedByDeleted))
	require.NotContains(t, s.records, id, "removed")
	assert.True(t, WasRemoved(id))

	stale := record(map[string]tombstone.Holder{"n2": pending(tombstone.HolderLive)})
	tombstone.Data.Set("n3", stale)
	s.handle(&msgbus.NodeTombstoneUpdated{Node: "n3", ID: id, Path: stale.Path, Value: stale})
	require.Contains(t, s.records, id, "answered")
	h := s.records[id].Holders["n2"]
	assert.Equal(t, tombstone.HolderCleared, h.State)
	assert.Equal(t, tombstone.ClearedByDeleted, h.ClearedBy, "as it was cleared")

	merged, _ := tombstone.Merge(stale, s.records[id])
	tombstone.Data.Set("n3", merged)
	s.handle(&msgbus.NodeTombstoneUpdated{Node: "n3", ID: id, Path: stale.Path, Value: merged})
	assert.NotContains(t, s.records, id, "removed again once the peer merged the answer")
	assert.Empty(t, tombstone.Data.PendingHolders("system/sec/s1", ""))
}

// A stale copy naming the node, the node removed it as no holder: an
// explicit drop clears it, and says what it cleared.
func TestDropStaleCopy(t *testing.T) {
	s, _ := newTestState(t)
	allUp("n1", "n2", "n3")
	s.gone[id] = goneRecord{at: time.Now()}
	tombstone.Data.Set("n2", record(map[string]tombstone.Holder{"n3": pending(tombstone.HolderLive)}))
	dropped, err := Drop("system/sec/s1")
	require.NoError(t, err)
	assert.Equal(t, []string{"n3"}, dropped)
	assert.Equal(t, tombstone.ClearedByDropped, s.records[id].Holders["n3"].ClearedBy)
}

// The records the peers publish with a pending holder and the node does
// not keep, missed as events, are adopted by the sweep.
func TestReconcile(t *testing.T) {
	s, _ := newTestState(t)
	allUp("n1")
	tombstone.Data.Set("n2", record(map[string]tombstone.Holder{"n1": pending(tombstone.HolderAway)}))
	tombstone.Data.Set("n3", tombstone.Record{OrchestrationID: "other", Path: "system/sec/s2", Kind: tombstone.KindDeleted,
		Holders: map[string]tombstone.Holder{"n1": {State: tombstone.HolderCleared, ClearedBy: tombstone.ClearedByDeleted}}})
	s.reconcile()
	assert.Contains(t, s.records, id)
	assert.NotContains(t, s.records, "other", "all cleared, not adopted")
}

// DemoteLocal turns away the live holder of the node in the record of the
// failed orchestration only, and takes in no record the node removed.
func TestDemoteLocal(t *testing.T) {
	s, _ := newTestState(t)
	allUp("n1")
	tombstone.Data.Set("n2", record(map[string]tombstone.Holder{"n1": pending(tombstone.HolderLive)}))
	other := tombstone.Record{OrchestrationID: "other", Path: "system/sec/s1", Kind: tombstone.KindDeleted,
		Holders: map[string]tombstone.Holder{"n1": pending(tombstone.HolderLive)}}
	tombstone.Data.Set("n3", other)
	require.NoError(t, DemoteLocal("system/sec/s1", id))
	assert.Equal(t, tombstone.HolderAway, s.records[id].Holders["n1"].Mode)
	assert.NotContains(t, s.records, "other", "another orchestration is left alone")

	s.gone["removed"] = goneRecord{at: time.Now()}
	tombstone.Data.Set("n3", tombstone.Record{OrchestrationID: "removed", Path: "system/sec/s1", Kind: tombstone.KindDeleted,
		Holders: map[string]tombstone.Holder{"n1": pending(tombstone.HolderLive)}})
	require.NoError(t, DemoteLocal("system/sec/s1", "removed"))
	assert.NotContains(t, s.records, "removed", "a record removed is not taken in again")
}

// Drop clears every pending holder of the path, adopting the copies of the
// records the node does not keep.
func TestDrop(t *testing.T) {
	s, _ := newTestState(t)
	tombstone.Data.Set("n2", record(map[string]tombstone.Holder{"n3": pending(tombstone.HolderAway)}))
	dropped, err := Drop("system/sec/s1")
	require.NoError(t, err)
	assert.Equal(t, []string{"n3"}, dropped)
	assert.Equal(t, tombstone.ClearedByDropped, s.records[id].Holders["n3"].ClearedBy)
	assert.Empty(t, tombstone.Data.PendingHolders("system/sec/s1", ""))
	l, err := tombstone.Load(s.dir)
	require.NoError(t, err)
	require.Len(t, l, 1, "written before Drop returns")
	assert.False(t, l[0].HasPending())
}

// The messages are published out of mu, in order, and Record returns once
// its own is published.
func TestPublishLoop(t *testing.T) {
	s, spy := newTestState(t)
	s.cond = sync.NewCond(&mu)
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.publishLoop()
	}()
	require.NoError(t, Record(record(map[string]tombstone.Holder{"n2": pending(tombstone.HolderLive)})))
	mu.Lock()
	n := len(spy.msgs)
	mu.Unlock()
	assert.Equal(t, 1, n)
	mu.Lock()
	s.demoteHolder("n2")
	s.stopped = true
	s.cond.Broadcast()
	mu.Unlock()
	<-done
	assert.Len(t, spy.msgs, 2, "the outbox drained on stop")
}

// A record not written makes Record fail, nothing deleted without its
// record, and is written at the next flush.
func TestFlushRetriesAWriteError(t *testing.T) {
	s, _ := newTestState(t)
	// A file where the directory is to be: no record can be written.
	require.NoError(t, os.WriteFile(s.dir, nil, 0o600))
	require.Error(t, Record(record(map[string]tombstone.Holder{"n2": pending(tombstone.HolderLive)})))
	assert.True(t, s.dirty[id], "to write again")

	require.NoError(t, os.Remove(s.dir))
	assert.Empty(t, flush())
	l, err := tombstone.Load(s.dir)
	require.NoError(t, err)
	assert.Len(t, l, 1, "written at the next flush")
}

// A record file not removed is an error of the flush, Drop's included, and
// removed at the next flush.
func TestFlushRetriesARemovalError(t *testing.T) {
	s, _ := newTestState(t)
	allUp("n1", "n2", "n3")
	require.NoError(t, Record(record(map[string]tombstone.Holder{"n3": pending(tombstone.HolderAway)})))
	// A directory with an entry where the file is: it can not be removed.
	p := filepath.Join(s.dir, id+".json")
	require.NoError(t, os.Remove(p))
	require.NoError(t, os.MkdirAll(filepath.Join(p, "x"), 0o700))

	_, err := Drop("system/sec/s1")
	require.Error(t, err, "the removal failed")
	assert.NotContains(t, s.records, id, "removed from memory")
	assert.True(t, s.removed[id], "to remove again")

	require.NoError(t, os.RemoveAll(p))
	assert.Empty(t, flush())
	assert.False(t, s.removed[id])
}
