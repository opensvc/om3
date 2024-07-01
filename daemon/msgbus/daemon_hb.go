package msgbus

func (data *ClusterData) onDaemonHb(m *DaemonHb) {
	data.Daemon.HB = m.Value
}
