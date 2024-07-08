package msgbus

import (
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	// ClusterData struct holds cluster data that can be updated with msg
	ClusterData struct {
		*cluster.Data

		localhost string
	}
)

func (data *ClusterData) ApplyMessage(m pubsub.Messager) {
	switch c := m.(type) {
	case *ClusterStatusUpdated:
		data.onClusterStatusUpdated(c)
	case *ClusterConfigUpdated:
		data.onClusterConfigUpdated(c)
	case *DaemonCollectorUpdated:
		data.onDaemonCollector(c)
	case *DaemonHeartbeatUpdated:
		data.onDaemonHeartbeatUpdated(c)
	case *DaemonListenerUpdated:
		data.onDaemonListenerUpdated(c)
	case *ForgetPeer:
		data.onForgetPeer(c)
	case *ObjectStatusDeleted:
		data.onObjectStatusDeleted(c)
	case *ObjectStatusUpdated:
		data.onObjectStatusUpdated(c)
	case *InstanceConfigDeleted:
		data.onInstanceConfigDeleted(c)
	case *InstanceConfigUpdated:
		data.onInstanceConfigUpdated(c)
	case *InstanceMonitorDeleted:
		data.onInstanceMonitorDeleted(c)
	case *InstanceMonitorUpdated:
		data.onInstanceMonitorUpdated(c)
	case *InstanceStatusDeleted:
		data.onInstanceStatusDeleted(c)
	case *InstanceStatusUpdated:
		data.onInstanceStatusUpdated(c)
	case *NodeConfigUpdated:
		data.onNodeConfigUpdated(c)
	case *NodeDataUpdated:
		data.onNodeDataUpdated(c)
	case *NodeMonitorDeleted:
		data.onNodeMonitorDeleted(c)
	case *NodeMonitorUpdated:
		data.onNodeMonitorUpdated(c)
	case *NodeOsPathsUpdated:
		data.onNodeOsPathsUpdated(c)
	case *NodeStatsUpdated:
		data.onNodeStatsUpdated(c)
	case *NodeStatusUpdated:
		data.onNodeStatusUpdated(c)
	}
}

func NewClusterData(cd *cluster.Data) *ClusterData {
	return &ClusterData{
		Data:      cd,
		localhost: cd.Daemon.Nodename,
	}
}
