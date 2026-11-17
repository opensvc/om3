package imon

import (
	"slices"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/tombmon"
)

// recordTombstoneFunc writes a tombstone, persisted before it returns: a
// variable for the tests to stub.
var recordTombstoneFunc = tombmon.Record

// noteCarrying records that the monitor of node was seen carrying the
// orchestration id of its global expect: a peer seen running the delete or
// the purge deletes its instance on its own.
func (t *Manager) noteCarrying(node string, mon instance.Monitor) {
	if mon.OrchestrationID == uuid.Nil {
		return
	}
	if t.seenCarrying == nil {
		t.seenCarrying = make(map[string]uuid.UUID)
	}
	t.seenCarrying[node] = mon.OrchestrationID
}

// recordTombstone writes, before the instance is deleted, the tombstone of
// the delete or the purge naming the scope nodes still holding the
// instance, or never seen running the orchestration: each pending until its
// instance is gone or kept, the others fetching no config from it, so the
// object does not come back from it. A node away is an away holder, its
// instance kept stopped on its return; a node up is a live holder, which
// completes the orchestration, as one whose config manager had not started
// yet as the delete ran, or one running it slower.
//
// A node running the delete is a holder until its instance is gone: killed
// before it deletes, it returns an away holder, where it brought the object
// back to the peers having deleted theirs. The last node to delete, its
// peers all gone, writes no record. An error fails the delete, nothing
// deleted without its record.
func (t *Manager) recordTombstone() error {
	var kind tombstone.Kind
	switch t.state.GlobalExpect {
	case instance.MonitorGlobalExpectPurged:
		kind = tombstone.KindPurged
	case instance.MonitorGlobalExpectDeleted:
		kind = tombstone.KindDeleted
	default:
		return nil
	}
	if t.state.OrchestrationID == uuid.Nil {
		return nil
	}
	holders := t.unseenHolders()
	if len(holders) == 0 {
		return nil
	}
	rec := tombstone.Record{
		OrchestrationID: t.state.OrchestrationID.String(),
		Path:            t.path.String(),
		Kind:            kind,
		PurgedAt:        t.state.GlobalExpectUpdatedAt,
		Holders:         holders,
	}
	t.log.Infof("record the tombstone of the %s, holders not seen running it: %v", kind, rec.PendingHolders(""))
	return recordTombstoneFunc(rec)
}

// isNodeUp says whether the node of a peer is known and runs object
// orchestrations, as the monitor last heard and as the node data says now:
// the data of a peer dropped goes before the event telling the monitor, its
// node monitor first, so a peer whose instance data is gone for a drop is
// never read up.
func (t *Manager) isNodeUp(nodename string) bool {
	mon, ok := t.nodeMonitor[nodename]
	if !ok || mon.State.IsAwayFromObjects() {
		return false
	}
	shared := node.MonitorData.GetByNode(nodename)
	return shared != nil && !shared.State.IsAwayFromObjects()
}

// unseenHolders returns the scope nodes, cluster nodes but this one, still
// holding the instance, or never seen running the orchestration of the
// monitor, as tombstone holders. A node seen running it with no instance
// left deleted it already.
func (t *Manager) unseenHolders() map[string]tombstone.Holder {
	id := t.state.OrchestrationID
	cluster := clusternode.Get()
	holding := instance.NodesWithInstance(t.path)
	holders := make(map[string]tombstone.Holder)
	for _, n := range t.scopeNodes {
		if n == t.localhost || !slices.Contains(cluster, n) {
			continue
		}
		if t.seenCarrying[n] == id && !holding[n] && t.isNodeUp(n) {
			// Seen running it, and its instance gone: deleted. A
			// node away or lost with no data left proves nothing,
			// its data dropped with it: it holds the record, as an
			// away holder, cleared on its return if its instance
			// is gone there.
			continue
		}
		h := tombstone.Holder{State: tombstone.HolderPending, Mode: tombstone.HolderLive}
		if !t.isNodeUp(n) {
			h.Mode = tombstone.HolderAway
		}
		holders[n] = h
	}
	return holders
}
