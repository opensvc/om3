package daemonapi

import (
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/daemondata"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
)

var (
	// announceAckTimeout bounds the wait of the peers acknowledging a node
	// state announced: the daemon stops or shuts down anyway, warning of
	// the peers that did not.
	announceAckTimeout = 5 * time.Second

	// announceAckPoll is the interval of the reads of the generations the
	// peers applied.
	announceAckPoll = 100 * time.Millisecond
)

func (a *DaemonAPI) announceSub(name string) {
	a.Bus.Pub(&msgbus.ClientSubscribed{Time: time.Now(), Name: name}, a.LabelLocalhost, labelOriginAPI)
}

func (a *DaemonAPI) announceUnsub(name string) {
	a.Bus.Pub(&msgbus.ClientUnsubscribed{Time: time.Now(), Name: name}, a.LabelLocalhost, labelOriginAPI)
}

// announceNodeState sets the node monitor state of the local node, and
// returns once the peers applied the data holding it, or after
// announceAckTimeout, warning of the peers that did not.
//
// A delay was waited instead, short of the time a daemon just started takes
// to exchange its data with its peers: a stop 337ms after a start went
// unannounced, the peers taking the node for down and failing its services
// over, instead of keeping them in maintenance (card 307).
func (a *DaemonAPI) announceNodeState(log *plog.Logger, state node.MonitorState) {
	log.Infof("announce node state %s", state)
	a.Bus.Pub(&msgbus.SetNodeMonitor{Node: a.localhost, Value: node.MonitorUpdate{State: &state}}, labelOriginAPI)
	if a.Daemondata == nil {
		time.Sleep(2 * daemondata.PropagationInterval())
		return
	}
	peers := waitedPeers(a.localhost, clusternode.Get(), daemonsubsystem.DataHeartbeat.Get(a.localhost))
	missing := waitAnnounceAck(a.Daemondata.GetAckGens, state, peers, announceAckTimeout, announceAckPoll)
	if len(missing) > 0 {
		log.Warnf("announce node state %s: not acknowledged within %s by %s", state, announceAckTimeout, strings.Join(missing, ", "))
	} else {
		log.Infof("announce node state %s: acknowledged by %d peer(s)", state, len(peers))
	}
}

// waitAnnounceAck waits the local data to hold the node monitor state, then
// the peers to apply the generation holding it, and returns the peers that
// did not by the timeout, or all of them when the local data did not hold
// the state.
func waitAnnounceAck(get func() daemondata.AckGens, state node.MonitorState, peers []string, timeout, poll time.Duration) []string {
	deadline := time.Now().Add(timeout)
	var target uint64
	for {
		gens := get()
		if target == 0 && gens.MonitorState == state {
			target = gens.Target()
		}
		if target > 0 {
			missing := unackedPeers(gens, target, peers)
			if len(missing) == 0 || !time.Now().Before(deadline) {
				return missing
			}
		} else if !time.Now().Before(deadline) {
			return peers
		}
		time.Sleep(poll)
	}
}

// unackedPeers returns the peers whose applied generation of the data of
// the local node is below target.
func unackedPeers(gens daemondata.AckGens, target uint64, peers []string) []string {
	var missing []string
	for _, peer := range peers {
		if gens.Peers[peer] < target {
			missing = append(missing, peer)
		}
	}
	return missing
}

// waitedPeers returns the cluster nodes, but the local one, an announce
// waits the acknowledgement of: all but those whose every heartbeat went
// stale. A peer not heard from yet, as by a daemon just started, is waited:
// its data is not exchanged yet, which is what the wait is for.
func waitedPeers(localhost string, nodes []string, hb *daemonsubsystem.Heartbeat) []string {
	var peers []string
	for _, peer := range nodes {
		if peer == localhost || peer == "" {
			continue
		}
		if hb != nil && wentStale(hb, peer) {
			continue
		}
		peers = append(peers, peer)
	}
	slices.Sort(peers)
	return peers
}

// wentStale is true when every receiving heartbeat stream of the peer heard
// it once and hears it no more. A watcher sets the time of the change of the
// beating state at its start, before the first beat: the time of the last
// beat tells a peer heard once from one not heard yet.
func wentStale(hb *daemonsubsystem.Heartbeat, peer string) bool {
	seen := false
	for _, stream := range hb.Streams {
		if !strings.HasSuffix(stream.ID, ".rx") {
			continue
		}
		status, ok := stream.Peers[peer]
		if !ok {
			continue
		}
		if status.IsBeating || status.LastBeatingAt.IsZero() {
			return false
		}
		seen = true
	}
	return seen
}
