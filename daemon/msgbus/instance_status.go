package msgbus

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/util/pubsub"
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

// instanceStatusUpdated returns []*InstanceStatusUpdated matching labels
func (data *ClusterData) instanceStatusUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	nodename := labels["node"]
	path := labels["path"]
	for n, nodeData := range data.Cluster.Node {
		if nodename != "" && nodename != n {
			continue
		}
		for instancePath, instanceData := range nodeData.Instance {
			if instanceData.Status == nil {
				continue
			}
			if path != "" && path != instancePath {
				continue
			}
			p, err := naming.ParsePath(instancePath)
			if err != nil {
				return nil, err
			}
			l = append(l, &InstanceStatusUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", n, "path", instancePath, "source", "cache"),
				},
				Path:  p,
				Node:  n,
				Value: *instanceData.Status.DeepCopy(),
			})
		}
	}
	return l, nil
}
