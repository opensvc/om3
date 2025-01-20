package msgbus

import "github.com/opensvc/om3/util/pubsub"

// onNodeConfigUpdated updates .cluster.node.<node>.config
func (data *ClusterData) onNodeConfigUpdated(m *NodeConfigUpdated) {
	newConfig := m.Value
	v := data.Cluster.Node[m.Node]
	if v.Config == newConfig {
		return
	}
	v.Config = m.Value
	data.Cluster.Node[m.Node] = v
}

func (data *ClusterData) nodeConfigUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			l = append(l, &NodeConfigUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Config.DeepCopy(),
			})
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			l = append(l, &NodeConfigUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Config.DeepCopy(),
			})
		}
	}
	return l, nil
}
