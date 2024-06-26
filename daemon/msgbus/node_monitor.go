package msgbus

import (
	"time"

	"github.com/opensvc/om3/core/node"
)

// onNodeMonitorDeleted reset .cluster.node.<node>.monitor with state shutting
// and IsPreserved true
func (data *ClusterData) onNodeMonitorDeleted(m *NodeMonitorDeleted) {
	if v, ok := data.Cluster.Node[m.Node]; ok {
		v.Monitor = node.Monitor{
			State:          node.MonitorStateShutting,
			IsPreserved:    true,
			StateUpdatedAt: time.Now(),
		}
		data.Cluster.Node[m.Node] = v
	}
}

// onNodeMonitorUpdated updates .cluster.node.<node>.monitor
func (data *ClusterData) onNodeMonitorUpdated(m *NodeMonitorUpdated) {
	newValue := data.Cluster.Node[m.Node]
	newValue.Monitor = m.Value
	data.Cluster.Node[m.Node] = newValue
}
