package msgbus

import "github.com/opensvc/om3/v3/util/pubsub"

func (data *ClusterData) onDaemonCollector(m *DaemonCollectorUpdated) {
	v := data.Cluster.Node[m.Node]
	v.Daemon.Collector = m.Value
	data.Cluster.Node[m.Node] = v
}

func (data *ClusterData) daemonCollector(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			l = append(l, &DaemonCollectorUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Daemon.Collector.DeepCopy(),
			})
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			l = append(l, &DaemonCollectorUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Daemon.Collector.DeepCopy(),
			})
		}
	}
	return l, nil
}
