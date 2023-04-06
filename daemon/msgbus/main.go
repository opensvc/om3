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

func (data *ClusterData) ApplyMessage(m pubsub.Messager) error {
	switch c := m.(type) {
	case *ClusterConfigUpdated:
		data.OnClusterConfigUpdated(c)
	case *DaemonHb:
		data.OnDaemonHb(c)
	case *ObjectStatusDeleted:
		data.onObjectStatusDeleted(c)
	case *ObjectStatusUpdated:
		data.onObjectStatusUpdated(c)
	case *InstanceConfigDeleted:
		data.OnInstanceConfigDeleted(c)
	case *InstanceConfigUpdated:
		data.OnInstanceConfigUpdated(c)
	case *InstanceMonitorDeleted:
		data.OnInstanceMonitorDeleted(c)
	case *InstanceMonitorUpdated:
		data.OnInstanceMonitorUpdated(c)
	case *InstanceStatusDeleted:
		data.OnInstanceStatusDeleted(c)
	case *InstanceStatusUpdated:
		data.OnInstanceStatusUpdated(c)
	case *NodeConfigUpdated:
		data.OnNodeConfigUpdated(c)
	case *NodeDataUpdated:
		data.onNodeDataUpdated(c)
	case *NodeMonitorDeleted:
		data.OnNodeMonitorDeleted(c)
	case *NodeMonitorUpdated:
		data.OnNodeMonitorUpdated(c)
	case *NodeOsPathsUpdated:
		data.OnNodeOsPathsUpdated(c)
	case *NodeStatsUpdated:
		data.OnNodeStatsUpdated(c)
	case *NodeStatusUpdated:
		data.OnNodeStatusUpdated(c)
	default:
	}
	return nil
}

func NewClusterData(cd *cluster.Data) *ClusterData {
	return &ClusterData{
		Data:         cd,
		localhost:    cd.Daemon.Nodename,
	}
}
