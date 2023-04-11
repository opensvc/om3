package msgbus

func (data *ClusterData) onDaemonHb(m *DaemonHb) {
	data.Daemon.Hb = m.Value
}
