package msgbus

import "github.com/opensvc/om3/util/pubsub"

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
