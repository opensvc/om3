package msgbus

// OnObjectStatusDeleted delete .cluster.object.<path>
func (data *ClusterData) OnObjectStatusDeleted(m *ObjectStatusDeleted) {
	delete(data.Cluster.Object, m.Path.String())
}

// OnObjectStatusUpdated updates .cluster.object.<path>
func (data *ClusterData) OnObjectStatusUpdated(m *ObjectStatusUpdated) {
	data.Cluster.Object[m.Path.String()] = m.Value
}
