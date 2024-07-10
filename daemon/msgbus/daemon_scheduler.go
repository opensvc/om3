package msgbus

func (data *ClusterData) onDaemonSchedulerUpdated(m *DaemonSchedulerUpdated) {
	v := data.Cluster.Node[m.Node]
	v.Daemon.Scheduler = m.Value
	data.Cluster.Node[m.Node] = v
}
