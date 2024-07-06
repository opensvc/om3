package msgbus

func (data *ClusterData) onDaemonHeartbeatUpdated(m *DaemonHeartbeatUpdated) {
	if ndata, ok := data.Cluster.Node[m.Node]; ok {
		ndata.Daemon.Heartbeat = m.Value
		data.Cluster.Node[m.Node] = ndata
	}
}
