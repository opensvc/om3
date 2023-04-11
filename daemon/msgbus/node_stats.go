package msgbus

// onNodeStatsUpdated updates .cluster.node.<node>.stats
func (data *ClusterData) onNodeStatsUpdated(m *NodeStatsUpdated) {
	v := data.Cluster.Node[m.Node]
	if v.Stats == m.Value {
		return
	}
	v.Stats = *m.Value.DeepCopy()
	data.Cluster.Node[m.Node] = v
}
