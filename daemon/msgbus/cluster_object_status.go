package msgbus

// onObjectStatusDeleted delete .cluster.object.<path>
func (data *ClusterData) onObjectStatusDeleted(m *ObjectStatusDeleted) {
	delete(data.Cluster.Object, m.Path.String())
}

// onObjectStatusUpdated updates .cluster.object.<path>
func (data *ClusterData) onObjectStatusUpdated(m *ObjectStatusUpdated) {
	data.Cluster.Object[m.Path.String()] = m.Value
}
