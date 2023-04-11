package msgbus

// onNodeMonitorDeleted removes .cluster.node.<node>.monitor
func (data *ClusterData) onNodeMonitorDeleted(m *NodeMonitorDeleted) {
	delete(data.Cluster.Node, m.Node)
}

// onNodeMonitorUpdated updates .cluster.node.<node>.monitor
func (data *ClusterData) onNodeMonitorUpdated(m *NodeMonitorUpdated) {
	newValue := data.Cluster.Node[m.Node]
	newValue.Monitor = m.Value
	data.Cluster.Node[m.Node] = newValue
}
