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

// starting says whether the collector holds its speech at its start.
func (t *T) starting() bool {
	return !t.startingUntil.IsZero()
}

// checkStarting ends the startup hold once the collector state of every live
// peer is known, or at its deadline.
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
	t.log.Infof("startup hold ended: the node may speak to the collector")
}

// peerStatesKnown says whether the collector state of every live peer is
// known.
func (t *T) peerStatesKnown() bool {
	for _, nodename := range clusternode.Get() {
		if nodename == t.localhost || node.MonitorData.GetByNode(nodename) == nil {
			continue
		}
		if c := daemonsubsystem.DataCollector.Get(nodename); c == nil || c.State == "" {
			return false
		}
	}
	return true
}

// startingPeerBefore returns the first live peer before the node in the
// cluster nodes whose collector is starting, "" when none is.
func (t *T) startingPeerBefore() string {
	for _, nodename := range clusternode.Get() {
		if nodename == t.localhost {
			return ""
		}
		if node.MonitorData.GetByNode(nodename) == nil {
			continue
		}
		if c := daemonsubsystem.DataCollector.Get(nodename); c != nil && c.State == "starting" {
			return nodename
		}
	}
	return ""
}
