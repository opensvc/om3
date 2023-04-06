package msgbus

// OnClusterStatusUpdated updates .cluster.status
func (data *ClusterData) OnClusterStatusUpdated(m *ClusterStatusUpdated) {
	data.Cluster.Status = m.Value
}
