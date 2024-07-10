package msgbus

func (data *ClusterData) onDaemonDataUpdated(m *DaemonDataUpdated) {
	v := data.Cluster.Node[m.Node]
	v.Daemon.Daemondata = m.Value
	data.Cluster.Node[m.Node] = v
}
