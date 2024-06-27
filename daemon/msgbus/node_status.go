package msgbus

import (
	"github.com/opensvc/om3/core/node"
)

// onNodeStatusUpdated updates .cluster.node.<node>.status from msgbus.NodeStatusUpdated and from gen cache.
// The gen cache contains synchronously updated gen values, and this may avoid undue path->full->patch message type
// transitions.
// TODO refactor or move this logic to the message producer ?
func (data *ClusterData) onNodeStatusUpdated(m *NodeStatusUpdated) {
	v := data.Cluster.Node[m.Node]
	gen := node.GenData.Get(m.Node)
	v.Status = m.Value
	if gen != nil {
		v.Status.Gen = *gen
	}
	data.Cluster.Node[m.Node] = v
}

func (data *ClusterData) onForgetPeer(m *ForgetPeer) {
	delete(data.Cluster.Node, m.Node)
}
