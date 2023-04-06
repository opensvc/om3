package msgbus

// OnClusterConfigUpdated sets .cluster.config
func (data *ClusterData) OnClusterConfigUpdated(c *ClusterConfigUpdated) {
	data.Cluster.Config = c.Value
}
