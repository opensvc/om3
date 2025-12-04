package msgbus

import (
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/util/pubsub"
)

func (data *ClusterData) onDaemonHeartbeatUpdated(m *DaemonHeartbeatUpdated) {
	if ndata, ok := data.Cluster.Node[m.Node]; ok {
		ndata.Daemon.Heartbeat = m.Value
		data.Cluster.Node[m.Node] = ndata
	}
}

func (data *ClusterData) onHeartbeatSecretUpdated(m *HeartbeatSecretUpdated) {
	if ndata, ok := data.Cluster.Node[m.Nodename]; ok {
		ndata.Daemon.Heartbeat.SecretVersion.Main = m.Value.MainVersion()
		ndata.Daemon.Heartbeat.SecretVersion.Alternate = m.Value.AltSecretVersion()
		data.Cluster.Node[m.Nodename] = ndata
	}
}

func (data *ClusterData) daemonHeartbeatUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			l = append(l, &DaemonHeartbeatUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Daemon.Heartbeat.DeepCopy(),
			})
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			l = append(l, &DaemonHeartbeatUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Daemon.Heartbeat.DeepCopy(),
			})
		}
	}
	return l, nil
}

func (data *ClusterData) heartbeatAlive(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)

	updateFromNodeData := func(nodename string, nodeData *node.Node) {
		for _, stream := range nodeData.Daemon.Heartbeat.Streams {
			for peerName, peerStatus := range stream.Peers {
				if peerStatus.IsBeating {
					l = append(l, &HeartbeatAlive{
						Msg: pubsub.Msg{
							Labels: pubsub.NewLabels("node", nodename, "hb", "alive/stale", "from", "cache"),
						},
						Nodename: peerName,
						HbID:     stream.ID,
						Time:     peerStatus.ChangedAt,
					})
				}
			}
		}
	}

	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			updateFromNodeData(nodename, &nodeData)
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			updateFromNodeData(nodename, &nodeData)
		}
	}

	return l, nil
}

func (data *ClusterData) heartbeatStale(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)

	updateFromNodeData := func(nodename string, nodeData *node.Node) {
		for _, stream := range nodeData.Daemon.Heartbeat.Streams {
			for peerName, peerStatus := range stream.Peers {
				if !peerStatus.IsBeating {
					l = append(l, &HeartbeatStale{
						Msg: pubsub.Msg{
							Labels: pubsub.NewLabels("node", nodename, "hb", "alive/stale", "from", "cache"),
						},
						Nodename: peerName,
						HbID:     stream.ID,
						Time:     peerStatus.ChangedAt,
					})
				}
			}
		}
	}

	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			updateFromNodeData(nodename, &nodeData)
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			updateFromNodeData(nodename, &nodeData)
		}
	}

	return l, nil
}
