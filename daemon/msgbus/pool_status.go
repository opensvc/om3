package msgbus

import (
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/util/pubsub"
)

// onPoolStatusUpdated updates .cluster.node.<node>.pool.<name>.status
func (data *ClusterData) onPoolStatusUpdated(c *NodePoolStatusUpdated) {
	value := c.Value.DeepCopy()
	cnode, ok := data.Cluster.Node[c.Node]
	if !ok {
		cnode = node.Node{}
	}
	if cnode.Pool == nil {
		cnode.Pool = make(map[string]pool.Status)
	}
	cnode.Pool[c.Name] = *value
	data.Cluster.Node[c.Node] = cnode
}

// poolStatusUpdated returns []PoolStatusUpdated events from cache
func (data *ClusterData) poolStatusUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			for poolName, p := range nodeData.Pool {
				l = append(l, &NodePoolStatusUpdated{
					Msg: pubsub.Msg{
						Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
					},
					Node:  nodename,
					Name:  poolName,
					Value: *p.DeepCopy(),
				})
			}
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			for poolName, p := range nodeData.Pool {
				l = append(l, &NodePoolStatusUpdated{
					Msg: pubsub.Msg{
						Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
					},
					Node:  nodename,
					Name:  poolName,
					Value: *p.DeepCopy(),
				})
			}
		}
	}
	return l, nil
}
