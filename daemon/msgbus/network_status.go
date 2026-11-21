package msgbus

import (
	"github.com/opensvc/om3/v3/core/netstatus"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// onNetworkStatusDeleted deletes .cluster.node.<node>.network.<name>
func (data *ClusterData) onNetworkStatusDeleted(c *NodeNetworkStatusDeleted) {
	if cnode, ok := data.Cluster.Node[c.Node]; ok && cnode.Network != nil {
		delete(cnode.Network, c.Name)
	}
}

// onNetworkStatusUpdated updates .cluster.node.<node>.network.<name>
func (data *ClusterData) onNetworkStatusUpdated(c *NodeNetworkStatusUpdated) {
	value := c.Value.DeepCopy()
	cnode, ok := data.Cluster.Node[c.Node]
	if !ok {
		cnode = node.Node{}
	}
	if cnode.Network == nil {
		cnode.Network = make(map[string]netstatus.Status)
	}
	cnode.Network[c.Name] = *value
	data.Cluster.Node[c.Node] = cnode
}

// networkStatusUpdated returns the NodeNetworkStatusUpdated events of the
// cache, of the node the labels name, or of every node.
func (data *ClusterData) networkStatusUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	for nodename, nodeData := range data.Cluster.Node {
		if want := labels["node"]; want != "" && want != nodename {
			continue
		}
		for name, st := range nodeData.Network {
			l = append(l, &NodeNetworkStatusUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Name:  name,
				Value: *st.DeepCopy(),
			})
		}
	}
	return l, nil
}
