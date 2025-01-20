package msgbus

import (
	"time"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/util/pubsub"
)

// onNodeMonitorDeleted reset .cluster.node.<node>.monitor with state shutting
// and IsPreserved true
func (data *ClusterData) onNodeMonitorDeleted(m *NodeMonitorDeleted) {
	if v, ok := data.Cluster.Node[m.Node]; ok {
		now := time.Now()
		v.Monitor = node.Monitor{
			State:          node.MonitorStateShutting,
			IsPreserved:    true,
			UpdatedAt:      now,
			StateUpdatedAt: now,
		}
		data.Cluster.Node[m.Node] = v
	}
}

// onNodeMonitorUpdated updates .cluster.node.<node>.monitor
func (data *ClusterData) onNodeMonitorUpdated(m *NodeMonitorUpdated) {
	newValue := data.Cluster.Node[m.Node]
	newValue.Monitor = m.Value
	data.Cluster.Node[m.Node] = newValue
}

func (data *ClusterData) nodeMonitorUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			l = append(l, &NodeMonitorUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Monitor.DeepCopy(),
			})
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			l = append(l, &NodeMonitorUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Monitor.DeepCopy(),
			})
		}
	}
	return l, nil
}
