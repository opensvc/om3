package msgbus

func (data *ClusterData) onDaemonListenerUpdated(m *DaemonListenerUpdated) {
	v := data.Cluster.Node[m.Node]
	v.Daemon.Listener = m.Value
	data.Cluster.Node[m.Node] = v
}
