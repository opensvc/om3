package msgbus

// OnNodeStatusUpdated updates .cluster.node.<node>.status from msgbus.NodeStatusUpdated.
func (data *ClusterData) OnNodeStatusUpdated(m *NodeStatusUpdated) {
	v := data.Cluster.Node[m.Node]
	v.Status = m.Value
	data.Cluster.Node[m.Node] = v
}
