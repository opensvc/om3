package nmon

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
		return true
	}
	return false
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
