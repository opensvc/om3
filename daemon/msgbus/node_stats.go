package msgbus

import "github.com/opensvc/om3/v3/util/pubsub"

// onNodeStatsUpdated updates .cluster.node.<node>.stats
func (data *ClusterData) onNodeStatsUpdated(m *NodeStatsUpdated) {
	v := data.Cluster.Node[m.Node]
	if v.Stats == m.Value {
		return
	}
	v.Stats = *m.Value.DeepCopy()
	data.Cluster.Node[m.Node] = v
}

func (data *ClusterData) nodeStatsUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			l = append(l, &NodeStatsUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Stats.DeepCopy(),
			})
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			l = append(l, &NodeStatsUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Stats.DeepCopy(),
			})
		}
	}
	return l, nil
}
