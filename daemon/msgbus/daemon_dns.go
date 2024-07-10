package msgbus

func (data *ClusterData) onDaemonDnsUpdated(m *DaemonDnsUpdated) {
	v := data.Cluster.Node[m.Node]
	v.Daemon.Dns = m.Value
	data.Cluster.Node[m.Node] = v
}
