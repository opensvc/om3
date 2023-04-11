package msgbus

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
)

// onInstanceStatusDeleted remove .cluster.node.<node>.instance.<path>.status
func (data *ClusterData) onInstanceStatusDeleted(c *InstanceStatusDeleted) {
	s := c.Path.String()
	if inst, ok := data.Cluster.Node[c.Node].Instance[s]; ok && inst.Status != nil {
		inst.Status = nil
		data.Cluster.Node[c.Node].Instance[s] = inst
	}
}

// onInstanceStatusUpdated updates .cluster.node.<node>.instance.<path>.status
func (data *ClusterData) onInstanceStatusUpdated(c *InstanceStatusUpdated) {
	s := c.Path.String()
	value := c.Value.DeepCopy()
	if cnode, ok := data.Cluster.Node[c.Node]; ok {
		if cnode.Instance == nil {
			cnode.Instance = make(map[string]instance.Instance)
		}
		inst := cnode.Instance[s]
		inst.Status = value
		cnode.Instance[s] = inst
		data.Cluster.Node[c.Node] = cnode
	} else {
		data.Cluster.Node[c.Node] = node.Node{Instance: map[string]instance.Instance{s: {Status: value}}}
	}
}
