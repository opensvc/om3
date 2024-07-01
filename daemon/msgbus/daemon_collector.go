package msgbus

func (data *ClusterData) onDaemonCollector(m *DaemonCollectorUpdated) {
	v := data.Cluster.Node[m.Node]
	v.Daemon.Collector = m.Value
	data.Cluster.Node[m.Node] = v
}
