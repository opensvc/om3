package msgbus

func (data *ClusterData) onDaemonCollector(m *DaemonCollector) {
	data.Daemon.Collector = m.Value
}
