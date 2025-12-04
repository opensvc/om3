package msgbus

import (
	"strings"

	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/util/pubsub"
)

func (data *ClusterData) nodeAlive(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	nodeM := make(map[string]struct{})
	updateFromNodeData := func(nodename string, streams []daemonsubsystem.HeartbeatStream) {
		for _, stream := range streams {
			if !strings.HasSuffix(stream.ID, ".rx") {
				continue
			}
			for peerName, peerStatus := range stream.Peers {
				k := nodename + "-" + peerName
				if _, ok := nodeM[k]; ok {
					continue
				}
				if peerStatus.IsBeating {
					nodeM[k] = struct{}{}
					l = append(l, &NodeAlive{
						Msg: pubsub.Msg{
							Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
						},
						Node: peerName,
					})
				}
			}
		}
	}

	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			updateFromNodeData(nodename, nodeData.Daemon.Heartbeat.Streams)
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			updateFromNodeData(nodename, nodeData.Daemon.Heartbeat.Streams)
		}
	}

	return l, nil
}

func (data *ClusterData) nodeStale(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	nodeM := make(map[string]struct{})

	updateFromNodeData := func(nodename string, streams []daemonsubsystem.HeartbeatStream) {
		for _, stream := range streams {
			if !strings.HasSuffix(stream.ID, ".rx") {
				continue
			}
			for peerName, peerStatus := range stream.Peers {
				if !peerStatus.IsBeating {
					k := nodename + "-" + peerName
					if _, ok := nodeM[k]; !ok {
						nodeM[k] = struct{}{}
						l = append(l, &NodeStale{
							Msg: pubsub.Msg{
								Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
							},
							Node: peerName,
						})
					}
				}
			}
		}
	}

	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			updateFromNodeData(nodename, nodeData.Daemon.Heartbeat.Streams)
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			updateFromNodeData(nodename, nodeData.Daemon.Heartbeat.Streams)
		}
	}
	return l, nil
}
