package msgbus

func (data *ClusterData) OnDaemonHb(m *DaemonHb) {
	data.Daemon.Hb = m.Value
}
