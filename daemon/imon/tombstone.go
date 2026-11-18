package imon

import (
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/tombmon"
	"github.com/opensvc/om3/v3/util/file"
)

// recordTombstoneFunc writes a tombstone, synced and published before it
// returns: a variable for the tests to stub.
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

// recordTombstone writes, before the instance is deleted, the point of no
// return of a delete or a purge, the record of the version of the config
// deleted, when a scope node may still hold it: one away, one not seen
// running the orchestration, as one whose config manager had not started
// yet, or one seen running it and still holding the instance, as one killed
// before it deleted. That version of the config, held anywhere, is then
// known as deleted: not fetched, not served, deleted by a node up that
// missed the orchestration, kept stopped with an issue on a node returning.
//
// No node may hold it, as for the last node to delete, its peers gone: no
// record. A delete adopted from a record writes none: that record does. An
// error fails the delete, nothing deleted without its record.
//
// It runs in the runner goroutine of crmDelete, reading the state of the
// monitor: the loop is parked meanwhile in runner.Run, which returns once
// the action is done, so nothing writes that state as it reads it.
func (t *Manager) recordTombstone() error {
	rec, ok := t.newTombstone()
	if !ok {
		return nil
	}
	return recordTombstoneFunc(rec)
}

// newTombstone returns the record recordTombstone writes, and whether one is
// to be written.
func (t *Manager) newTombstone() (tombstone.Record, bool) {
	var kind tombstone.Kind
	switch t.state.GlobalExpect {
	case instance.MonitorGlobalExpectPurged:
		kind = tombstone.KindPurged
	case instance.MonitorGlobalExpectDeleted:
		kind = tombstone.KindDeleted
	default:
		return tombstone.Record{}, false
	}
	if t.state.OrchestrationID == uuid.Nil || t.adoptedTombstone == t.state.OrchestrationID.String() {
		return tombstone.Record{}, false
	}
	nodes := t.mayHold()
	if len(nodes) == 0 {
		return tombstone.Record{}, false
	}
	version := t.instConfig.UpdatedAt
	if version.IsZero() {
		version = file.ModTime(t.path.ConfigFile())
	}
	rec := tombstone.Record{
		OrchestrationID: t.state.OrchestrationID.String(),
		Path:            t.path.String(),
		Kind:            kind,
		ObjectID:        t.instConfig.ID,
		Version:         version,
		DeletedAt:       t.state.GlobalExpectUpdatedAt,
		Nodes:           nodes,
	}
	t.log.Infof("record the tombstone of the %s of version %s, nodes that may still hold it: %v", kind, version.Format(time.RFC3339Nano), nodes)
	return rec, true
}

// mayHold returns the scope nodes, cluster nodes but this one, that may
// still hold the instance: all but the ones seen running the orchestration
// and holding nothing any more, their node up. A node away or lost with no
// data left proves nothing, its data dropped with it.
func (t *Manager) mayHold() []string {
	id := t.state.OrchestrationID
	cluster := clusternode.Get()
	holding := instance.NodesWithInstance(t.path)
	var l []string
	for _, n := range t.scopeNodes {
		if n == t.localhost || !slices.Contains(cluster, n) {
			continue
		}
		if t.seenCarrying[n] == id && !holding[n] && t.isNodeUp(n) {
			continue
		}
		l = append(l, n)
	}
	slices.Sort(l)
	return l
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

// refreshTombstones reads the records killing the local config, of any
// node, and looks at the orchestration again.
func (t *Manager) refreshTombstones() {
	t.killers = nil
	if !t.instConfig.UpdatedAt.IsZero() {
		t.killers = tombstone.Data.KilledBy(t.path.String(), t.instConfig.UpdatedAt)
	}
	t.applyTombstoneIssue()
	t.onChange()
}

// adoptableKiller returns the record whose delete or purge the instance
// takes on: trusted, heard live while the node was up, and of an
// orchestration the instance did not run already, as one that failed or was
// aborted, which adopted again would run what the operator stopped.
func (t *Manager) adoptableKiller() (tombstone.Killer, bool) {
	for _, k := range t.killers {
		if !k.Trusted {
			continue
		}
		if oid, err := uuid.Parse(k.Record.OrchestrationID); err != nil || t.carried[oid] || k.Record.OrchestrationID == t.adoptedTombstone {
			continue
		}
		return k, true
	}
	return tombstone.Killer{}, false
}

// adoptTombstone takes on the delete or the purge of a record killing the
// local config, heard live, when no orchestration runs: the instance was
// not seen running it, as one whose config manager had not started yet. A
// record read as the node returned is not adopted: the node was away, and
// may hold the only copy of the data.
//
// The global expect, its stamp and the orchestration id are the ones of the
// record, as a peer monitor gives them. It says whether it adopted one.
func (t *Manager) adoptTombstone() bool {
	if t.state.GlobalExpect != instance.MonitorGlobalExpectNone || t.state.OrchestrationID != uuid.Nil {
		return false
	}
	if t.nodeMonitor[t.localhost].State != node.MonitorStateIdle {
		return false
	}
	k, ok := t.adoptableKiller()
	if !ok {
		return false
	}
	oid, err := uuid.Parse(k.Record.OrchestrationID)
	if err != nil {
		return false
	}
	ge := instance.MonitorGlobalExpectPurged
	if k.Record.Kind == tombstone.KindDeleted {
		ge = instance.MonitorGlobalExpectDeleted
	}
	t.log.Infof("adopt the %s of the tombstone %s of node %s, killing the version of the config here -> global expect %s", k.Record.Kind, k.Record.OrchestrationID, k.Node, ge)
	t.change = true
	t.state.GlobalExpect = ge
	t.state.GlobalExpectUpdatedAt = k.Record.DeletedAt
	t.state.GlobalExpectOptions = nil
	t.state.OrchestrationID = oid
	t.state.OrchestrationIsDone = false
	t.state.State = instance.MonitorStateIdle
	t.globalExpectSeenAt = time.Now()
	t.adoptedTombstone = k.Record.OrchestrationID
	t.logSetOrchestrationID(oid)
	return true
}

// tombstoneIssue returns the issue of the instance when a record kills its
// config and no delete of it is to come here, and the id of the record, or
// empty: the object was purged or deleted on the other nodes while this one
// was away, or its delete failed or was aborted here. The instance may hold
// the only copy of the data: the operator purges or deletes it here, or
// keeps it by an edit or a touch of its config, a newer version no record
// kills, which then propagates again.
func (t *Manager) tombstoneIssue() (string, string) {
	if len(t.killers) == 0 {
		return "", ""
	}
	switch t.state.GlobalExpect {
	case instance.MonitorGlobalExpectPurged, instance.MonitorGlobalExpectDeleted:
		// Its delete runs.
		return "", ""
	}
	if _, ok := t.adoptableKiller(); ok {
		return "", ""
	}
	rec := t.killers[0].Record
	action := "purge"
	if rec.Kind == tombstone.KindDeleted {
		action = "delete"
	}
	return fmt.Sprintf("%s at %s, this version of the config deleted: run om %s %s, or edit or touch its config to keep it",
		rec.Kind, rec.DeletedAt.Format(time.RFC3339), t.path, action), rec.OrchestrationID
}

// applyTombstoneIssue shows the issue in the instance monitor, and flags the
// instance stopped once per record, unless it is up: the daemon does not
// start it on its own, and the operator decides. The instance is never
// stopped here, and the frozen flag, the operator's, is never touched.
func (t *Manager) applyTombstoneIssue() {
	issue, id := t.tombstoneIssue()
	var issues []string
	if issue != "" {
		issues = []string{issue}
	}
	if !slices.Equal(issues, t.state.Issues) {
		t.state.Issues = issues
		t.change = true
	}
	if id == "" || t.tombstoneStopped == id {
		return
	}
	if t.instStatus[t.localhost].UpdatedAt.IsZero() {
		// Not known yet whether the instance is up: decided on its
		// first status.
		return
	}
	if t.instStatus[t.localhost].Avail.Is(status.Up, status.Warn) {
		t.tombstoneStopped = id
		return
	}
	t.log.Infof("flag the instance stopped: %s", issue)
	if err := t.setStopped(); err != nil {
		// Decided again at the next status or tombstone event.
		return
	}
	t.tombstoneStopped = id
}

// refusedByTombstone says whether a global expect is refused to an instance
// a tombstone blocks: what starts or provisions it. A purge, a delete, a
// stop and an unprovision are accepted.
func refusedByTombstone(ge instance.MonitorGlobalExpect) bool {
	switch ge {
	case instance.MonitorGlobalExpectStarted, instance.MonitorGlobalExpectRestarted,
		instance.MonitorGlobalExpectPlaced, instance.MonitorGlobalExpectPlacedAt,
		instance.MonitorGlobalExpectProvisioned:
		return true
	default:
		return false
	}
}
