package imon

import (
	"time"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

const (
	// outdatedMargin is added to the time the local instance status goes
	// outdated, so the refresh runs past it, not right before.
	outdatedMargin = time.Second

	// outdatedRefreshFloor is the least time between two refreshes asked
	// by the outdated timer, so a status outdating right away does not
	// have the daemon refresh it in a loop.
	outdatedRefreshFloor = 30 * time.Second

	// stateFileRefreshDelay is how long after a state file written by a
	// peer the status is refreshed, for the files written at once to cause
	// one refresh.
	stateFileRefreshDelay = 2 * time.Second

	// peerChangeRefreshDelay is how long after a peer instance status
	// change a local status depending on it is refreshed, for the changes a
	// peer action publishes at once to cause one refresh.
	peerChangeRefreshDelay = 2 * time.Second
)

// outdatedRefreshDelay is how long from now to refresh a status outdated at,
// the last refresh the timer asked being at last.
func outdatedRefreshDelay(at, last, now time.Time) time.Duration {
	due := at.Add(outdatedMargin)
	if floor := last.Add(outdatedRefreshFloor); due.Before(floor) {
		due = floor
	}
	if d := due.Sub(now); d > 0 {
		return d
	}
	return 0
}

// armOutdatedTimer arms the timer refreshing the local instance status when
// it goes outdated with no event to tell, as a copy aging past its delay. A
// zero time disarms it.
func (t *Manager) armOutdatedTimer(at time.Time) {
	t.outdatedTimer.Stop()
	if at.IsZero() {
		return
	}
	t.outdatedTimer.Reset(outdatedRefreshDelay(at, t.outdatedRefreshedAt, time.Now()))
}

// onInstanceStateFileUpdated refreshes the local instance status when a peer
// wrote a state file it reads: the source of a sync telling it synced this
// node, which makes a copy found stale fresh again, with no timer to tell.
//
// A sync writes a few state files at once, so the refresh is delayed a little,
// for them to cause one.
func (t *Manager) onInstanceStateFileUpdated(c *msgbus.InstanceStateFileUpdated) {
	t.log.Debugf("refresh the status in %s, state file %s updated", stateFileRefreshDelay, c.File)
	t.peerRefreshTimer.Reset(stateFileRefreshDelay)
}

// refreshOnPeerChange refreshes the local instance status when a peer
// instance changed in a way a local resource status depends on: a resource
// saying its status depends on the peers, whose peer counterpart changed
// status, or whose peer instance changed availability, as a peer started or
// stopped. A peer taking or dropping a reservation, or swapping the roles of
// a replicated device, changes nothing local that would tell.
//
// Only a status value change triggers it, not a new log line or timestamp,
// so two nodes depending on each other stop refreshing once their statuses
// settle.
func (t *Manager) refreshOnPeerChange(peer string, prev, cur instance.Status) {
	local, ok := t.instStatus[t.localhost]
	if !ok {
		return
	}
	if what, changed := peerChangeSeenBy(local, prev, cur); changed {
		t.log.Debugf("refresh the status in %s, %s changed on %s", peerChangeRefreshDelay, what, peer)
		t.peerRefreshTimer.Reset(peerChangeRefreshDelay)
	}
}

// peerChangeSeenBy tells what, from prev to cur, changed in a peer instance
// that a resource of the local instance status depends on.
func peerChangeSeenBy(local, prev, cur instance.Status) (string, bool) {
	dependent := false
	for rid, rs := range local.Resources {
		if !rs.DependsOnPeers {
			continue
		}
		dependent = true
		if prev.Resources[rid].Status != cur.Resources[rid].Status {
			return rid, true
		}
	}
	if dependent && prev.Avail != cur.Avail {
		return "the availability", true
	}
	return "", false
}

// onPeerRefreshTimer refreshes the local instance status after a peer event
// it depends on. An action in progress refreshes it when done.
func (t *Manager) onPeerRefreshTimer() {
	if t.state.State != instance.MonitorStateIdle {
		t.log.Tracef("skip the refresh asked by a peer event: state %s", t.state.State)
		return
	}
	t.log.Debugf("refresh the status, asked by a peer event")
	t.requestStatusRefresh(t.instConfig.Priority)
}

// onOutdatedTimer refreshes the local instance status gone outdated. An
// action in progress refreshes it when done, which arms the timer again.
func (t *Manager) onOutdatedTimer() {
	if t.state.State != instance.MonitorStateIdle {
		t.log.Tracef("skip the refresh of the outdated status: state %s", t.state.State)
		return
	}
	t.outdatedRefreshedAt = time.Now()
	t.log.Debugf("refresh the status, outdated")
	t.requestStatusRefresh(t.instConfig.Priority)
}
