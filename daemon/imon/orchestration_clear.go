package imon

import (
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/errcontext"
)

var (
	// orchestrationClearTimeout bounds the wait for the peers to clear an
	// orchestration ended here, before its end is announced anyway: a
	// peer gone does not hold the requester.
	orchestrationClearTimeout = 10 * time.Second

	// heldRequestTimeout bounds the hold of a request for a new
	// orchestration while the one before it is not cleared.
	heldRequestTimeout = 10 * time.Second
)

// endOrchestrationWhenPeersCleared holds back the end of the orchestration
// this node accepted until every peer cleared it.
//
// The end, which ends the wait of the requester, was announced as this node
// cleared the orchestration, its peers clearing their part a heartbeat
// later: a next request asked at once to a peer was refused as the
// orchestration "already in progress".
func (t *Manager) endOrchestrationWhenPeersCleared() {
	if t.orchestrationPending == nil {
		return
	}
	t.publishOrchestrationEnding()
	t.orchestrationEnding = t.orchestrationPending
	t.orchestrationPending = nil
	t.orchestrationEndingSince = time.Now()
	t.onOrchestrationClearCheck()
}

// peersHolding returns the peers whose instance monitor carries the
// orchestration id.
func (t *Manager) peersHolding(id string) []string {
	var l []string
	for node, instMon := range t.instMonitor {
		if node == t.localhost {
			continue
		}
		if instMon.OrchestrationID != uuid.Nil && instMon.OrchestrationID.String() == id {
			l = append(l, node)
		}
	}
	slices.Sort(l)
	return l
}

// publishOrchestrationEnding announces the end held back.
func (t *Manager) publishOrchestrationEnding() {
	if t.orchestrationEnding == nil {
		return
	}
	t.publisher.Pub(t.orchestrationEnding, t.pubLabels...)
	t.orchestrationEnding = nil
}

// holdRequest holds a request for a new orchestration received while the
// orchestration of this monitor is done here, and not cleared as its peers
// are not seen done yet, rather than refuse it as already in progress: it
// is taken on once the orchestration clears, as v2 took a new request once
// the instances were idle. The requester is answered the request queued.
func (t *Manager) holdRequest(c *msgbus.SetInstanceMonitor) bool {
	v := c.Value
	switch {
	case v.GlobalExpect == nil, v.State != nil, v.LocalExpect != nil:
		return false
	case *v.GlobalExpect == instance.MonitorGlobalExpectAborted:
		return false
	case v.CandidateOrchestrationID == uuid.Nil:
		return false
	case t.orchestrationEnding != nil:
		// Done here, its id cleared, its end held back while a peer
		// still runs it: a request accepted now would end it early.
	case t.state.OrchestrationID == uuid.Nil, !t.state.OrchestrationIsDone:
		return false
	}
	if t.heldRequest != nil {
		t.refuseHeldRequest("superseded by a newer request")
	}
	t.log.Infof("hold the request for %s until the orchestration %s clears on every node", *v.GlobalExpect, t.state.OrchestrationID)
	t.heldRequest = c
	t.heldRequestSince = time.Now()
	t.onOrchestrationClearCheck()
	return true
}

// refuseHeldRequest refuses the request held, for the reason.
func (t *Manager) refuseHeldRequest(reason string) {
	c := t.heldRequest
	if c == nil {
		return
	}
	t.heldRequest = nil
	err := fmt.Errorf("%s", reason)
	t.log.Infof("refuse the request held for %s: %s", *c.Value.GlobalExpect, reason)
	if v, ok := c.Err.(errcontext.ErrCloseSender); ok {
		v.Send(err)
		v.Close()
	}
	t.publisher.Pub(&msgbus.ObjectOrchestrationRefused{
		Node:         t.localhost,
		Path:         t.path,
		ID:           c.Value.CandidateOrchestrationID.String(),
		Reason:       reason,
		GlobalExpect: c.Value.GlobalExpect,
	}, t.pubLabels...)
}

func (t *Manager) onOrchestrationClearTimer() {
	t.onOrchestrationClearCheck()
	// A served mark may have expired, which the check above deletes: the
	// orchestration waiting for that peer is looked at again, as it waited
	// for good, nothing else bringing it back (qatests 203.03).
	t.onChange()
	t.updateIfChange()
}

// onOrchestrationClearCheck announces the end held back once the peers
// cleared the orchestration or the wait timed out, takes on the request
// held once the orchestration cleared here, refuses it on timeout, and arms
// the timer for the next deadline. The deadlines are of the local clock.
func (t *Manager) onOrchestrationClearCheck() {
	now := time.Now()
	if t.orchestrationEnding != nil {
		holding := t.peersHolding(t.orchestrationEnding.ID)
		switch {
		case len(holding) == 0:
			t.publishOrchestrationEnding()
		case now.Sub(t.orchestrationEndingSince) >= orchestrationClearTimeout:
			t.log.Warnf("orchestration %s ended: announced, its peers %s not seen clearing it within %s", t.orchestrationEnding.ID, holding, orchestrationClearTimeout)
			t.publishOrchestrationEnding()
		}
	}
	if c := t.heldRequest; c != nil {
		switch {
		case t.state.OrchestrationID == uuid.Nil && t.orchestrationEnding == nil:
			t.heldRequest = nil
			t.log.Infof("take on the request held for %s: the orchestration before it cleared", *c.Value.GlobalExpect)
			t.onSetInstanceMonitor(c)
		case now.Sub(t.heldRequestSince) >= heldRequestTimeout:
			t.refuseHeldRequest(fmt.Sprintf("orchestration %s (>%s) already in progress", t.state.OrchestrationID, t.state.GlobalExpect))
		}
	}
	t.armOrchestrationClearTimer(now)
}

func (t *Manager) armOrchestrationClearTimer(now time.Time) {
	if t.orchestrationClearTimer == nil {
		return
	}
	t.orchestrationClearTimer.Stop()
	var (
		next  time.Duration
		armed bool
	)
	consider := func(d time.Duration) {
		if !armed || d < next {
			next, armed = d, true
		}
	}
	for peer, at := range t.servedAt {
		d := servedStartingTimeout - now.Sub(at)
		if d < 0 {
			delete(t.servedAt, peer)
			continue
		}
		consider(d)
	}
	if t.orchestrationEnding != nil {
		consider(orchestrationClearTimeout - now.Sub(t.orchestrationEndingSince))
	}
	if t.heldRequest != nil {
		consider(heldRequestTimeout - now.Sub(t.heldRequestSince))
	}
	if !armed {
		return
	}
	if next < 0 {
		next = 0
	}
	t.orchestrationClearTimer.Reset(next)
}
