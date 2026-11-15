package daemonapi

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/daemondata"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
)

// fakeAckGens returns the generations of a daemon whose local data holds the
// state announced at the second read, committed at the third, then applied
// by the peers as given by peersAt.
type fakeAckGens struct {
	mu      sync.Mutex
	reads   int
	state   node.MonitorState
	peersAt func(read int) map[string]uint64
}

func (f *fakeAckGens) get() daemondata.AckGens {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	gens := daemondata.AckGens{Local: 10, MonitorState: node.MonitorStateIdle, Peers: f.peersAt(f.reads)}
	switch {
	case f.reads == 2:
		gens.MonitorState = f.state
		gens.Pending = true
	case f.reads > 2:
		gens.MonitorState = f.state
		gens.Local = 11
	}
	return gens
}

// An announce returns once every peer waited applied the generation holding
// the state, the commit pending when the state was seen included (card 307).
func TestWaitAnnounceAck(t *testing.T) {
	f := &fakeAckGens{state: node.MonitorStateMaintenance, peersAt: func(read int) map[string]uint64 {
		if read < 5 {
			// The peers applied the generation before the state.
			return map[string]uint64{"n2": 10, "n3": 10}
		}
		return map[string]uint64{"n2": 11, "n3": 12}
	}}
	missing := waitAnnounceAck(f.get, node.MonitorStateMaintenance, []string{"n2", "n3"}, time.Second, time.Millisecond)
	assert.Empty(t, missing)
	assert.Equal(t, 5, f.reads)
}

// A peer that does not apply the generation by the timeout is returned, the
// announce going on anyway.
func TestWaitAnnounceAckTimeout(t *testing.T) {
	f := &fakeAckGens{state: node.MonitorStateMaintenance, peersAt: func(int) map[string]uint64 {
		return map[string]uint64{"n2": 11}
	}}
	begin := time.Now()
	missing := waitAnnounceAck(f.get, node.MonitorStateMaintenance, []string{"n2", "n3"}, 50*time.Millisecond, time.Millisecond)
	assert.Equal(t, []string{"n3"}, missing, "n3 not heard from")
	assert.Less(t, time.Since(begin), time.Second)
}

// A state the local data never holds returns every peer by the timeout.
func TestWaitAnnounceAckStateNotApplied(t *testing.T) {
	f := &fakeAckGens{state: node.MonitorStateShutdownProgress, peersAt: func(int) map[string]uint64 {
		return map[string]uint64{"n2": 99}
	}}
	missing := waitAnnounceAck(f.get, node.MonitorStateMaintenance, []string{"n2"}, 20*time.Millisecond, time.Millisecond)
	assert.Equal(t, []string{"n2"}, missing)
}

// The peers waited are the cluster nodes but the local one and those whose
// every heartbeat went stale. A peer never heard from, as by a daemon just
// started, is waited, though its watcher set the time of a change at its
// start (n4).
func TestWaitedPeers(t *testing.T) {
	now := time.Now()
	hb := &daemonsubsystem.Heartbeat{Streams: []daemonsubsystem.HeartbeatStream{
		{Status: daemonsubsystem.Status{ID: "hb#1.rx"}, Peers: map[string]daemonsubsystem.HeartbeatStreamPeerStatus{
			"n2": {IsBeating: true, ChangedAt: now, LastBeatingAt: now},
			"n3": {IsBeating: false, ChangedAt: now, LastBeatingAt: now},
			"n4": {IsBeating: false, ChangedAt: now},
			"n5": {IsBeating: false, ChangedAt: now, LastBeatingAt: now},
		}},
		{Status: daemonsubsystem.Status{ID: "hb#1.tx"}, Peers: map[string]daemonsubsystem.HeartbeatStreamPeerStatus{
			"n3": {IsBeating: true, ChangedAt: now, LastBeatingAt: now},
		}},
		{Status: daemonsubsystem.Status{ID: "hb#2.rx"}, Peers: map[string]daemonsubsystem.HeartbeatStreamPeerStatus{
			"n3": {IsBeating: false, ChangedAt: now, LastBeatingAt: now},
			"n5": {IsBeating: true, ChangedAt: now, LastBeatingAt: now},
		}},
	}}
	nodes := []string{"n1", "n2", "n3", "n4", "n5", "n6"}
	assert.Equal(t, []string{"n2", "n4", "n5", "n6"}, waitedPeers("n1", nodes, hb))
	assert.Equal(t, []string{"n2", "n3", "n4", "n5", "n6"}, waitedPeers("n1", nodes, nil), "no heartbeat status yet")
}
