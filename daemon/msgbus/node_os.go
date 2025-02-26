package msgbus

import (
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/san"
)

// onNodeOsPathsUpdated updates .cluster.node.<node>.os.paths
func (data *ClusterData) onNodeOsPathsUpdated(m *NodeOsPathsUpdated) {
	v := data.Cluster.Node[m.Node]
	v.Os.Paths = m.Value
	data.Cluster.Node[m.Node] = v
}

func (data *ClusterData) nodeOsPathsUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			l = append(l, &NodeOsPathsUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: append([]san.Path{}, nodeData.Os.Paths...),
			})
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			l = append(l, &NodeOsPathsUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: append([]san.Path{}, nodeData.Os.Paths...),
			})
		}
	}
	return l, nil
}
