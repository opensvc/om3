package msgbus

import "github.com/opensvc/om3/util/pubsub"

func (data *ClusterData) ExtractEvents(m any, labels pubsub.Labels) ([]any, error) {
	switch m.(type) {
	case *ClusterStatusUpdated:
		return data.clusterStatusUpdated(labels)
	case *ClusterConfigUpdated:
		return data.clusterConfigUpdated(labels)
	case *DaemonCollectorUpdated:
		return data.daemonCollector(labels)
	case *DaemonDataUpdated:
		return data.daemonDataUpdated(labels)
	case *DaemonDnsUpdated:
		return data.daemonDnsUpdated(labels)
	case *DaemonHeartbeatUpdated:
		return data.daemonHeartbeatUpdated(labels)
	case *DaemonListenerUpdated:
		return data.daemonListenerUpdated(labels)
	case *DaemonRunnerImonUpdated:
		return data.daemonRunnerImonUpdated(labels)
	case *DaemonSchedulerUpdated:
		return data.daemonSchedulerUpdated(labels)
	case *ObjectStatusUpdated:
		return data.objectStatusUpdated(labels)
	case *InstanceConfigUpdated:
		return data.instanceConfigUpdated(labels)
	case *InstanceMonitorUpdated:
		return data.instanceMonitorUpdated(labels)
	case *InstanceStatusUpdated:
		return data.instanceStatusUpdated(labels)
	case *NodeConfigUpdated:
		return data.nodeConfigUpdated(labels)
	case *NodeDataUpdated:
		return data.nodeDataUpdated(labels)
	case *NodeMonitorUpdated:
		return data.nodeMonitorUpdated(labels)
	case *NodeOsPathsUpdated:
		return data.nodeOsPathsUpdated(labels)
	case *NodeStatsUpdated:
		return data.nodeStatsUpdated(labels)
	case *NodeStatusUpdated:
		return data.nodeStatusUpdated(labels)
	}
	return nil, nil
}
