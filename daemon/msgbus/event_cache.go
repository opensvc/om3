package msgbus

import "github.com/opensvc/om3/v3/util/pubsub"

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
	case *HeartbeatAlive:
		return data.heartbeatAlive(labels)
	case *HeartbeatStale:
		return data.heartbeatStale(labels)
	case *ObjectStatusUpdated:
		return data.objectStatusUpdated(labels)
	case *NodePoolStatusUpdated:
		return data.poolStatusUpdated(labels)
	case *InstanceConfigUpdated:
		return data.instanceConfigUpdated(labels)
	case *InstanceMonitorUpdated:
		return data.instanceMonitorUpdated(labels)
	case *InstanceStatusUpdated:
		return data.instanceStatusUpdated(labels)
	case *NodeConfigUpdated:
		return data.nodeConfigUpdated(labels)
	case *NodeAlive:
		return data.nodeAlive(labels)
	case *NodeDataUpdated:
		return data.nodeDataUpdated(labels)
	case *NodeMonitorUpdated:
		return data.nodeMonitorUpdated(labels)
	case *NodeOsPathsUpdated:
		return data.nodeOsPathsUpdated(labels)
	case *NodeStatsUpdated:
		return data.nodeStatsUpdated(labels)
	case *NodeStale:
		return data.nodeStale(labels)
	case *NodeStatusUpdated:
		return data.nodeStatusUpdated(labels)
	case *NodeStatusArbitratorsUpdated:
		return data.nodeStatusArbitratorsUpdated(labels)
	}
	return nil, nil
}

var (
	// ExtractableEvents Extractable events for event cache
	// Try to keep this in sync with the list of events in the msgbus package that
	// can be extracted.
	//
	// Keep ordered with NodeDataUpdated first, then with other events in the natural
	// emission order.
	ExtractableEvents = [24]any{
		&NodeDataUpdated{},
		&NodePoolStatusUpdated{},
		&NodeAlive{},
		&NodeStale{},

		&NodeConfigUpdated{},
		&NodeMonitorUpdated{},
		&NodeOsPathsUpdated{},
		&NodeStatsUpdated{},
		&NodeStatusUpdated{},

		&ClusterConfigUpdated{},
		&ClusterStatusUpdated{},

		&DaemonDataUpdated{},
		&DaemonHeartbeatUpdated{},
		&DaemonCollectorUpdated{},
		&DaemonDnsUpdated{},
		&DaemonListenerUpdated{},
		&DaemonRunnerImonUpdated{},
		&DaemonSchedulerUpdated{},

		&HeartbeatAlive{},
		&HeartbeatStale{},

		&InstanceConfigUpdated{},
		&InstanceMonitorUpdated{},
		&InstanceStatusUpdated{},

		&ObjectStatusUpdated{},
	}
)
