package msgbus

// OnNodeStatsUpdated updates .cluster.node.<node>.stats
func (data *ClusterData) OnNodeStatsUpdated(m *NodeStatsUpdated) {
	v := data.Cluster.Node[m.Node]
	if v.Stats == m.Value {
		return
	}
	v.Stats = *m.Value.DeepCopy()
	data.Cluster.Node[m.Node] = v
}
