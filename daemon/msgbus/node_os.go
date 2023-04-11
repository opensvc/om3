package msgbus

// onNodeOsPathsUpdated updates .cluster.node.<node>.os.paths
func (data *ClusterData) onNodeOsPathsUpdated(m *NodeOsPathsUpdated) {
	v := data.Cluster.Node[m.Node]
	v.Os.Paths = m.Value
	data.Cluster.Node[m.Node] = v
}
