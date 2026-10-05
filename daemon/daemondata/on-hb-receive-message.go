package daemondata

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/opensvc/om3/v3/core/hbtype"
	"github.com/opensvc/om3/v3/core/node"
)

func (d *data) onReceiveHbMsg(msg *hbtype.Msg) {
	if !d.acceptPeerRun(msg) {
		return
	}
	switch msg.Kind {
	case "patch":
		d.setFromPeerMsg(msg.Nodename, msg.Kind, len(msg.Events), msg.Gen)
		if err := d.applyMsgEvents(msg); err != nil {
			d.log.Errorf("apply message %s events from %s gens: %v: %s", msg.Kind, msg.Nodename, msg.Gen, err)
		}
		// cleanup previous applied full info
		delete(d.previousRemoteInfo, msg.Nodename)
		onReceiveQueueOperationTotal.With(prometheus.Labels{"operation": "patch"}).Inc()

	case "full":
		d.setFromPeerMsg(msg.Nodename, msg.Kind, 0, msg.Gen)
		if d.hbGens[d.localNode][msg.Nodename] == msg.Gen[msg.Nodename] {
			// already have most recent version of peer
			d.log.Tracef("onReceiveHbMsg skipped %s from %s gens: %v (already have peer gen applied)", msg.Kind, msg.Nodename, msg.Gen)
			return
		}
		if d.hbGens[d.localNode][msg.Nodename]+uint64(len(msg.Events)) >= msg.Gen[msg.Nodename] {
			// We can apply events instead of full
			previouslyApplied := d.hbGens[d.localNode][msg.Nodename]
			if err := d.applyMsgEvents(msg); err != nil {
				d.log.Errorf("apply message %s events from %s gens: %v (previously applied peer gen %d, local gens: %+v): %s",
					msg.Kind, msg.Nodename, msg.Gen,
					previouslyApplied, d.hbGens, err)
			}
			if d.hbGens[d.localNode][msg.Nodename] == msg.Gen[msg.Nodename] {
				// the events have been applied => node data not needed
				d.log.Tracef("apply message %s events from %s gens: %v succeed (previously applied peer %d now %d)",
					msg.Kind, msg.Nodename, msg.Gen,
					previouslyApplied, msg.Gen[msg.Nodename])
				return
			}
		}
		if err := d.applyNodeData(msg); err != nil {
			d.log.Errorf("apply message %s node data from %s gens: %v: %s", msg.Kind, msg.Nodename, msg.Gen, err)
		}
		onReceiveQueueOperationTotal.With(prometheus.Labels{"operation": "full"}).Inc()
	case "ping":
		d.setFromPeerMsg(msg.Nodename, msg.Kind, 0, msg.Gen)
		// cleanup previous applied full info
		delete(d.previousRemoteInfo, msg.Nodename)
		onReceiveQueueOperationTotal.With(prometheus.Labels{"operation": "ping"}).Inc()
	}
}

func (d *data) setFromPeerMsg(peer, msgType string, length int, gen node.Gen) {
	d.setHbMsgType(peer, msgType)
	d.setHbMsgPatchLength(peer, length)
	d.hbGens[peer] = gen
	if gen[d.localNode] != d.hbGens[d.localNode][d.localNode] {
		d.needMsg = true
	}
	if gen[peer] != d.hbGens[d.localNode][peer] {
		d.needMsg = true
	}
}

const (
	// peerRunConfirmCount is the number of messages, received with
	// increasing stamps, that confirm a run of a peer stamped before its
	// known run: a message of a previous run delivered late comes once per
	// heartbeat at most, a live run keeps sending.
	peerRunConfirmCount = 3

	// peerRunCandidateExpire is the local time after which a run candidate
	// not seen again is forgotten, so late messages of a previous run
	// spread over time do not add up to a confirmation.
	peerRunCandidateExpire = 30 * time.Second
)

// acceptPeerRun tracks the daemon run of the peer msg is from, and returns
// false when msg must be dropped.
//
// A run stamped after the known one means the peer restarted: what this
// node applied of it belongs to the previous run, so the peer is asked a
// full message. A restart is otherwise detected only when the gen of the
// peer goes back below the one applied, which this node does not see when
// it misses the first messages of the new run, as when it restarts at the
// same time: it would apply the events of the new run on top of the data of
// the previous one.
//
// A run stamped before the known one proves nothing alone: it may be a
// message of a previous run delivered late, which must not be applied, or a
// new run of a peer whose clock was stepped back. Its messages are dropped
// until peerRunConfirmCount of them came with increasing stamps, then the
// peer is taken as restarted. The stamps compared are all written by the
// clock of the peer, so the clock of this node does not matter, nor the
// time zones, nor a DST change.
func (d *data) acceptPeerRun(msg *hbtype.Msg) bool {
	if msg.RunID == "" {
		// a peer not telling its run
		return true
	}
	peer := msg.Nodename
	known, ok := d.peerRuns[peer]
	switch {
	case !ok:
		d.peerRuns[peer] = peerRun{id: msg.RunID, updatedAt: msg.UpdatedAt}
		return true
	case known.id == msg.RunID:
		if msg.UpdatedAt.After(known.updatedAt) {
			d.peerRuns[peer] = peerRun{id: msg.RunID, updatedAt: msg.UpdatedAt}
		}
		delete(d.peerRunCandidates, peer)
		return true
	case msg.UpdatedAt.After(known.updatedAt):
		d.log.Infof("peer %s daemon restarted (run %s -> %s): ask its full data", peer, known.id, msg.RunID)
		d.switchPeerRun(msg)
		return true
	}

	// msg.RunID is not the same as the known run
	now := time.Now()
	c, ok := d.peerRunCandidates[peer]
	switch {
	case !ok, c.id != msg.RunID, now.Sub(c.seenAt) > peerRunCandidateExpire:
		c = peerRunCandidate{id: msg.RunID, updatedAt: msg.UpdatedAt, count: 1, seenAt: now}
	case msg.UpdatedAt.After(c.updatedAt):
		// already candidate, with increasing timestamp
		c.count++
		c.updatedAt = msg.UpdatedAt
		c.seenAt = now
	}
	if c.count >= peerRunConfirmCount {
		d.log.Infof("peer %s daemon restarted with its clock stepped back (run %s -> %s): ask its full data", peer, known.id, msg.RunID)
		d.switchPeerRun(msg)
		return true
	}
	d.peerRunCandidates[peer] = c
	d.log.Debugf("drop msg %s from %s: run %s stamped before the known run %s (%d/%d)", msg.Kind, peer, msg.RunID, known.id, c.count, peerRunConfirmCount)
	return false
}

// switchPeerRun records the run of msg as the run of its peer, and asks the
// peer a full message.
func (d *data) switchPeerRun(msg *hbtype.Msg) {
	peer := msg.Nodename
	d.hbGens[d.localNode][peer] = 0
	delete(d.hbPatchMsgUpdated, peer)
	delete(d.previousRemoteInfo, peer)
	delete(d.peerRunCandidates, peer)
	d.peerRuns[peer] = peerRun{id: msg.RunID, updatedAt: msg.UpdatedAt}
}
