package msgbus

// onClusterConfigUpdated sets .cluster.config
func (data *ClusterData) onClusterConfigUpdated(c *ClusterConfigUpdated) {
	data.Cluster.Config = c.Value
}
