package nmon

import "github.com/opensvc/om3/v3/daemon/locktable"

// updateSpeaker refresh the t.nodeStatus.IsLeader value. It returns true
// if value have been changed, and we have to publish NodeStatusUpdated.
//
// It must be called during following events:
//
//	onStartup, onConfigFileUpdated, onClusterConfigUpdated: clusterConfig.Nodes have been changed
//	onForgetPeer, onPeerNodeMonitorUpdated: live peer list has been updated
//
// onStartup value may be true, because live peers are not yet discovered
func (t *Manager) updateSpeaker() bool {
	if t.isSpeakerNode() != t.nodeStatus.IsLeader {
		t.nodeStatus.IsLeader = !t.nodeStatus.IsLeader
		t.log.Infof("node speaker: %v", t.nodeStatus.IsLeader)
		// The cluster locks are granted from the table of the node
		// speaking. One that starts speaking rebuilds it from what the
		// clients of the cluster hold, as the node speaking before granted
		// locks it never saw, and one that stops has a table to forget.
		locktable.SpeakerTable.Drop()
		return true
	}
	return false
}

// updateSpeakerAndPublish updates the speaker, and publishes the node status
// when it changed: the subscribers, as the collector choosing its speaker from
// it, see the change then, rather than on the next node status published for
// another reason.
func (t *Manager) updateSpeakerAndPublish() {
	if t.updateSpeaker() {
		t.publishNodeStatus()
	}
}

// isSpeakerNode return true if we are the speaker node
func (t *Manager) isSpeakerNode() bool {
	return t.speakerNode() == t.localhost
}

// speakerNode return the speaker node: the first alive node from
// clusterConfig.Nodes.
func (t *Manager) speakerNode() string {
	for _, nodename := range t.clusterConfig.Nodes {
		if _, ok := t.livePeers[nodename]; ok {
			return nodename
		}
	}
	return ""
}
