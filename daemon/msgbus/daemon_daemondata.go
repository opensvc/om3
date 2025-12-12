package msgbus

import (
	"github.com/opensvc/om3/v3/util/pubsub"
)

func (data *ClusterData) onDaemonDataUpdated(m *DaemonDataUpdated) {
	v := data.Cluster.Node[m.Node]
	v.Daemon.Daemondata = m.Value
	data.Cluster.Node[m.Node] = v
}

func (data *ClusterData) daemonDataUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			l = append(l, &DaemonDataUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Daemon.Daemondata.DeepCopy(),
			})
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			l = append(l, &DaemonDataUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Daemon.Daemondata.DeepCopy(),
			})
		}
	}
	return l, nil
}
