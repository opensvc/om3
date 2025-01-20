package msgbus

import "github.com/opensvc/om3/util/pubsub"

// onClusterConfigUpdated sets .cluster.config
func (data *ClusterData) onNodeDataUpdated(c *NodeDataUpdated) {
	data.Cluster.Node[c.Node] = c.Value
}

func (data *ClusterData) nodeDataUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			l = append(l, &NodeDataUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.DeepCopy(),
			})
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			l = append(l, &NodeDataUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.DeepCopy(),
			})
		}
	}
	return l, nil
}
