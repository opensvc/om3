package imon

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/tombmon"
)

var (
	// recordTombstoneFunc writes a tombstone, persisted before it
	// returns: a variable for the tests to stub.
	recordTombstoneFunc = tombmon.Record

	// demoteTombstoneFunc turns away the live holder of the node in a
	// tombstone of a path: a variable for the tests to stub.
	demoteTombstoneFunc = tombmon.DemoteLocal
)

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
	if t.adoptedTombstone == t.state.OrchestrationID.String() {
		// Adopted from a tombstone: the record names the holders.
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

// refreshTombstones reads the tombstones of the object, the copies of every
// node merged, and looks at the orchestration again: a live holder of the
// node adopts its delete or purge.
func (t *Manager) refreshTombstones() {
	t.tombstones = tombstone.Data.MergedOf(t.path.String())
	t.applyTombstoneIssue()
	t.onChange()
}

// adoptTombstone takes on the delete or the purge of a tombstone naming the
// node a live holder, when no orchestration runs: the instance was not seen
// running it, as one whose config manager had not started yet, and the node
// accepting it, its monitor gone, can no longer be adopted from. It is the
// orchestration completing, not a catch-up on a return: a node away, or
// restarted since, is an away holder, and never deletes on its own.
//
// The global expect, its stamp and the orchestration id are the ones of the
// record, as a peer monitor gives them. It says whether it adopted one.
func (t *Manager) adoptTombstone() bool {
	if t.state.GlobalExpect != instance.MonitorGlobalExpectNone || t.state.OrchestrationID != uuid.Nil {
		return false
	}
	if t.nodeMonitor[t.localhost].State != node.MonitorStateIdle {
		// Not before the rejoin: the record merged then turns the
		// holder of the node away.
		return false
	}
	for id, rec := range t.tombstones {
		h, ok := rec.Holders[t.localhost]
		if !ok || h.State != tombstone.HolderPending || h.Mode != tombstone.HolderLive {
			continue
		}
		oid, err := uuid.Parse(id)
		if err != nil || id == t.adoptedTombstone {
			continue
		}
		if tombmon.WasRemoved(id) {
			// A stale copy a peer publishes again: the node cleared
			// its holder, and tombmon answers it.
			continue
		}
		if oid == t.carriedOrchestrationID {
			// The instance ran it already, and failed or was
			// aborted: adopting it again would run what the
			// operator stopped. The holder turns away instead, the
			// issue shown, and the operator decides.
			if t.tombstoneDemoted != id {
				t.tombstoneDemoted = id
				t.log.Infof("the tombstone %s names this node a live holder of an orchestration it ran already: turn the holder away", id)
				if err := demoteTombstoneFunc(t.path.String(), id); err != nil {
					t.log.Warnf("turn the live holder of the tombstone %s away: %s", id, err)
				}
			}
			continue
		}
		ge := instance.MonitorGlobalExpectPurged
		if rec.Kind == tombstone.KindDeleted {
			ge = instance.MonitorGlobalExpectDeleted
		}
		t.log.Infof("adopt the %s of the tombstone %s naming this node a live holder -> global expect %s", rec.Kind, id, ge)
		t.change = true
		t.state.GlobalExpect = ge
		t.state.GlobalExpectUpdatedAt = rec.PurgedAt
		t.state.GlobalExpectOptions = nil
		t.state.OrchestrationID = oid
		t.state.OrchestrationIsDone = false
		t.state.State = instance.MonitorStateIdle
		t.globalExpectSeenAt = time.Now()
		t.adoptedTombstone = id
		t.logSetOrchestrationID(oid)
		return true
	}
	return false
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

// awayInstancesRefusal returns why a purge, a delete or an unprovision of
// the object is refused, or empty: the nodes away whose instance, as they
// last published it, is up or provisioned, provisioned only for an
// unprovision. The orchestration does not wait for a node away: a purge or
// a delete can not release its instance, maybe the only copy of the data,
// which it holds a tombstone for; an unprovision may release shared
// resources that instance still uses. A node with no status known needs no
// force: nothing says it holds anything.
func (t *Manager) awayInstancesRefusal(ge instance.MonitorGlobalExpect) string {
	var l []string
	for _, n := range t.scopeNodes {
		if n == t.localhost {
			continue
		}
		if mon, ok := t.nodeMonitor[n]; ok && !mon.State.IsAwayFromObjects() {
			continue
		}
		st, ok := t.instStatus[n]
		if !ok {
			if st, ok = t.awayStatus[n]; !ok {
				continue
			}
		}
		holds := st.Provisioned == provisioned.True || st.Provisioned == provisioned.Mixed
		if ge != instance.MonitorGlobalExpectUnprovisioned {
			holds = holds || st.Avail.Is(status.Up, status.Warn)
		}
		if holds {
			l = append(l, n)
		}
	}
	if len(l) == 0 {
		return ""
	}
	slices.Sort(l)
	switch ge {
	case instance.MonitorGlobalExpectUnprovisioned:
		return fmt.Sprintf("%s away with an instance provisioned: the unprovision would not wait for it, and may release resources it still uses: use --force to unprovision without it", strings.Join(l, ", "))
	case instance.MonitorGlobalExpectDeleted:
		return fmt.Sprintf("%s away with an instance up or provisioned: the delete can not reach it, maybe the only copy of the data: use --force to delete without it, the node holding a tombstone", strings.Join(l, ", "))
	default:
		return fmt.Sprintf("%s away with an instance up or provisioned: the purge can not release it, maybe the only copy of the data: use --force to purge without it, the node holding a tombstone", strings.Join(l, ", "))
	}
}

// noteAwayStatus remembers the last status of the instance of node, deleted
// while its node is away: a node stopping deletes the statuses of its
// instances as its monitors stop, after it announced its shutdown, so a
// purge, a delete or an unprovision would else not know what it holds. It
// is forgotten as the node publishes a status again.
func (t *Manager) noteAwayStatus(node string, st instance.Status) {
	if mon, ok := t.nodeMonitor[node]; ok && !mon.State.IsAwayFromObjects() {
		return
	}
	if t.awayStatus == nil {
		t.awayStatus = make(map[string]instance.Status)
	}
	t.awayStatus[node] = st
}

// forceAway says whether the options of a purge, a delete or an
// unprovision ask it to go on without the instances of the nodes away.
func forceAway(options any) bool {
	switch o := options.(type) {
	case instance.MonitorGlobalExpectOptionsPurged:
		return o.Force
	case instance.MonitorGlobalExpectOptionsDeleted:
		return o.Force
	case instance.MonitorGlobalExpectOptionsUnprovisioned:
		return o.Force
	default:
		return false
	}
}

// tombstoneIssue returns the issue of the instance when a tombstone names its
// node an away holder, and the id of the record, or empty: the object was
// purged or deleted on the other nodes while this one was away, and this
// instance may hold the only copy of the data. The operator purges or
// deletes it here, or keeps it by an edit or a touch of its config, which
// clears the holder and propagates the config again.
func (t *Manager) tombstoneIssue() (string, string) {
	for id, rec := range t.tombstones {
		h, ok := rec.Holders[t.localhost]
		if !ok || h.State != tombstone.HolderPending || h.Mode != tombstone.HolderAway {
			continue
		}
		action := "purge"
		if rec.Kind == tombstone.KindDeleted {
			action = "delete"
		}
		return fmt.Sprintf("%s on the other nodes at %s while this node was away: run om %s %s, or edit or touch its config to keep it",
			rec.Kind, rec.PurgedAt.Format(time.RFC3339), t.path, action), id
	}
	return "", ""
}

// applyTombstoneIssue shows the issue of an away holder in the instance
// monitor, and flags the instance stopped once per record, unless it is up:
// the daemon does not start it on its own, and the operator decides. The
// instance is never stopped here, and the frozen flag, the operator's, is
// never touched.
func (t *Manager) applyTombstoneIssue() {
	t.demoteTombstoneIfFailed()
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

// refusedByTombstone says whether a global expect is refused to an away
// holder: what starts or provisions the instance. A purge, a delete, a stop
// and an unprovision are accepted.
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

// demoteTombstoneIfFailed turns away the live holder of the node, once per
// orchestration, when its instance failed the delete or the purge: the
// peers deleted theirs, so the object lives on this node only, and the
// operator decides, the issue shown, as for a node that was away.
func (t *Manager) demoteTombstoneIfFailed() {
	// The failures ending the orchestration only: a delete goes on after
	// a stop, a start or a freeze failed, as from idle.
	switch t.state.GlobalExpect {
	case instance.MonitorGlobalExpectPurged:
		if !t.state.State.IsOneOf(instance.MonitorStateStopFailure, instance.MonitorStatePurgeFailed) {
			return
		}
	case instance.MonitorGlobalExpectDeleted:
		if t.state.State != instance.MonitorStateDeleteFailure {
			return
		}
	default:
		return
	}
	id := t.state.OrchestrationID.String()
	rec, ok := t.tombstones[id]
	if !ok || t.tombstoneDemoted == id {
		return
	}
	if h := rec.Holders[t.localhost]; h.State != tombstone.HolderPending || h.Mode != tombstone.HolderLive {
		return
	}
	t.tombstoneDemoted = id
	t.log.Infof("the %s failed here, the peers done: turn the live holder of the tombstone %s away", rec.Kind, id)
	if err := demoteTombstoneFunc(t.path.String(), id); err != nil {
		t.log.Warnf("turn the live holder of the tombstone %s away: %s", id, err)
	}
}
