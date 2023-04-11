package msgbus

// onNodeConfigUpdated updates .cluster.node.<node>.config
func (data *ClusterData) onNodeConfigUpdated(m *NodeConfigUpdated) {
	newConfig := m.Value
	v := data.Cluster.Node[m.Node]
	if v.Config == newConfig {
		return
	}
	v.Config = m.Value
	data.Cluster.Node[m.Node] = v
}
