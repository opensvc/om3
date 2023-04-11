package msgbus

// onClusterConfigUpdated sets .cluster.config
func (data *ClusterData) onNodeDataUpdated(c *NodeDataUpdated) {
	data.Cluster.Node[c.Node] = c.Value
}
