package msgbus

import "github.com/opensvc/om3/util/pubsub"

func (data *ClusterData) onDaemonDnsUpdated(m *DaemonDnsUpdated) {
	v := data.Cluster.Node[m.Node]
	v.Daemon.Dns = m.Value
	data.Cluster.Node[m.Node] = v
}


func (data *ClusterData) daemonDnsUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			l = append(l, &DaemonDnsUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Daemon.Dns.DeepCopy(),
			})
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			l = append(l, &DaemonDnsUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Daemon.Dns.DeepCopy(),
			})
		}
	}
	return l, nil
}
