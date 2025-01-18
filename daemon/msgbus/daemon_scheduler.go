package msgbus

import "github.com/opensvc/om3/util/pubsub"

func (data *ClusterData) onDaemonSchedulerUpdated(m *DaemonSchedulerUpdated) {
	v := data.Cluster.Node[m.Node]
	v.Daemon.Scheduler = m.Value
	data.Cluster.Node[m.Node] = v
}


func (data *ClusterData) daemonSchedulerUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			l = append(l, &DaemonSchedulerUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Daemon.Scheduler.DeepCopy(),
			})
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			l = append(l, &DaemonSchedulerUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Daemon.Scheduler.DeepCopy(),
			})
		}
	}
	return l, nil
}
