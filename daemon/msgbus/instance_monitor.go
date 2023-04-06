package msgbus

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
)

// OnInstanceMonitorDeleted delete .cluster.node.<node>.instance.<path>.monitor
func (data *ClusterData) OnInstanceMonitorDeleted(m *InstanceMonitorDeleted) {
	s := m.Path.String()
	if inst, ok := data.Cluster.Node[m.Node].Instance[s]; ok && inst.Monitor != nil {
		inst.Monitor = nil
		data.Cluster.Node[m.Node].Instance[s] = inst
	}
}

// OnInstanceMonitorUpdated updates .cluster.node.<node>.instance.<path>.monitor
func (data *ClusterData) OnInstanceMonitorUpdated(m *InstanceMonitorUpdated) {
	s := m.Path.String()
	value := &m.Value
	if cnode, ok := data.Cluster.Node[m.Node]; ok {
		if cnode.Instance == nil {
			cnode.Instance = make(map[string]instance.Instance)
		}
		inst := cnode.Instance[s]
		inst.Monitor = value
		cnode.Instance[s] = inst
		data.Cluster.Node[m.Node] = cnode
	} else {
		data.Cluster.Node[m.Node] = node.Node{Instance: map[string]instance.Instance{s: {Monitor: value}}}
	}
}
