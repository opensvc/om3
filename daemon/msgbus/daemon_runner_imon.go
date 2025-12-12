package msgbus

import "github.com/opensvc/om3/v3/util/pubsub"

func (data *ClusterData) onDaemonRunnerImonUpdated(m *DaemonRunnerImonUpdated) {
	v := data.Cluster.Node[m.Node]
	v.Daemon.RunnerImon = m.Value
	data.Cluster.Node[m.Node] = v
}

func (data *ClusterData) daemonRunnerImonUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			l = append(l, &DaemonRunnerImonUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: nodeData.Daemon.RunnerImon,
			})
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			l = append(l, &DaemonRunnerImonUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: nodeData.Daemon.RunnerImon,
			})
		}
	}
	return l, nil
}
