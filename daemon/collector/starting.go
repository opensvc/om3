package collector

import (
	"time"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
)

// startingMax bounds the startup hold of the collector: past it, the node
// may speak though the collector state of a live peer is still unknown. It
// is far below the wait of a cluster enroll for the collector state of the
// enrolled node.
const startingMax = 15 * time.Second

// starting says whether the collector holds its speech, at its start or at
// its enabling.
func (t *T) starting() bool {
	return !t.startingUntil.IsZero()
}

// startHold starts holding the speech, for startingMax at most. onEnable is
// set for a collector a configuration change enables, as a collector.url set
// in the cluster config, which enables the collectors of the peers too: a
// peer saying disabled may not have applied the change yet, and is waited
// for as a peer whose state is not known.
func (t *T) startHold(onEnable bool) {
	t.startingUntil = time.Now().Add(startingMax)
	t.holdOnEnable = onEnable
}

// checkStarting ends the startup hold once the collector state of every live
// peer is known, or at its deadline. A hold on enable takes a peer saying
// disabled for one whose state is not known yet.
//
// A node starting, as one restarting first in the cluster nodes, would
// otherwise take the speech at once, beside the node speaking in its
// absence, which keeps it until it hears the state of the node starting. Its
// state says starting meanwhile, which the peers do not take for able to
// speak.
func (t *T) checkStarting(now time.Time) {
	if !t.starting() {
		return
	}
	if now.Before(t.startingUntil) && !t.peerStatesKnown() {
		return
	}
	t.startingUntil = time.Time{}
	t.holdOnEnable = false
	t.log.Debugf("startup hold ended: the node may speak to the collector")
}

// peerStatesKnown says whether the collector state of every live peer is
// known.
func (t *T) peerStatesKnown() bool {
	for _, nodename := range clusternode.Get() {
		if nodename == t.localhost || node.MonitorData.GetByNode(nodename) == nil {
			continue
		}
		c := daemonsubsystem.DataCollector.Get(nodename)
		if c == nil || c.State == "" || isStopState(c.State) {
			// A state stopping or stopped may be one the peer left
			// before a restart: wait for the state of its new daemon.
			return false
		}
		if t.holdOnEnable && c.State == "disabled" {
			return false
		}
	}
	return true
}

// isStopState says whether a collector state is the one of a collector
// stopping, as with its daemon restarting.
func isStopState(state string) bool {
	return state == "stopping" || state == "stopped"
}

// startingPeerBefore returns the first live peer before the node in the
// cluster nodes whose collector is starting, "" when none is. withStopped
// takes a peer stopping or stopped for one starting again, as for the first
// choice of a node restarting with it: a node choosing again has a speaker,
// and waits for it gone for collector.speaker_grace_period only.
func (t *T) startingPeerBefore(withStopped bool) string {
	for _, nodename := range clusternode.Get() {
		if nodename == t.localhost {
			return ""
		}
		if node.MonitorData.GetByNode(nodename) == nil {
			continue
		}
		c := daemonsubsystem.DataCollector.Get(nodename)
		if c != nil && (c.State == "starting" || (withStopped && isStopState(c.State))) {
			return nodename
		}
	}
	return ""
}
