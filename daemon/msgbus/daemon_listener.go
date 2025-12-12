package msgbus

import "github.com/opensvc/om3/v3/util/pubsub"

func (data *ClusterData) onDaemonListenerUpdated(m *DaemonListenerUpdated) {
	v := data.Cluster.Node[m.Node]
	v.Daemon.Listener = m.Value
	data.Cluster.Node[m.Node] = v
}

func (data *ClusterData) daemonListenerUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			l = append(l, &DaemonListenerUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Daemon.Listener.DeepCopy(),
			})
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			l = append(l, &DaemonListenerUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Daemon.Listener.DeepCopy(),
			})
		}
	}
	return l, nil
}
