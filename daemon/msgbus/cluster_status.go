package msgbus

// onClusterStatusUpdated updates .cluster.status
func (data *ClusterData) onClusterStatusUpdated(m *ClusterStatusUpdated) {
	data.Cluster.Status = m.Value
}
