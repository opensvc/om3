package msgbus

// OnNodeMonitorDeleted removes .cluster.node.<node>.monitor
func (data *ClusterData) OnNodeMonitorDeleted(m *NodeMonitorDeleted) {
	delete(data.Cluster.Node, m.Node)
}

// OnNodeMonitorUpdated updates .cluster.node.<node>.monitor
func (data *ClusterData) OnNodeMonitorUpdated(m *NodeMonitorUpdated) {
	newValue := data.Cluster.Node[m.Node]
	newValue.Monitor = m.Value
	data.Cluster.Node[m.Node] = newValue
}
