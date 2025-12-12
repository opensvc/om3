package msgbus

import (
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// onNodeStatusUpdated updates .cluster.node.<node>.status from msgbus.NodeStatusUpdated and from gen cache.
// The gen cache contains synchronously updated gen values, and this may avoid undue path->full->patch message type
// transitions.
// TODO refactor or move this logic to the message producer ?
func (data *ClusterData) onNodeStatusUpdated(m *NodeStatusUpdated) {
	v := data.Cluster.Node[m.Node]
	gen := node.GenData.GetByNode(m.Node)
	v.Status = m.Value
	if gen != nil {
		v.Status.Gen = *gen
	}
	data.Cluster.Node[m.Node] = v
}

func (data *ClusterData) onForgetPeer(m *ForgetPeer) {
	delete(data.Cluster.Node, m.Node)
}

func (data *ClusterData) nodeStatusUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			l = append(l, &NodeStatusUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Status.DeepCopy(),
			})
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			l = append(l, &NodeStatusUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
				},
				Node:  nodename,
				Value: *nodeData.Status.DeepCopy(),
			})
		}
	}
	return l, nil
}
