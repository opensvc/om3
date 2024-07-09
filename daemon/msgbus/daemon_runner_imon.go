package msgbus

func (data *ClusterData) onDaemonRunnerImonUpdated(m *DaemonRunnerImonUpdated) {
	v := data.Cluster.Node[m.Node]
	v.Daemon.RunnerImon = m.Value
	data.Cluster.Node[m.Node] = v
}
