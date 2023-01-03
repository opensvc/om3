package daemondata

import (
	"context"

	"github.com/goccy/go-json"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/hbcache"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	opDropPeerNode struct {
		err  chan<- error
		node string
	}

	opGetNodeData struct {
		node   string
		result chan<- *cluster.NodeData
	}
)

// DropPeerNode drops cluster.node.<peer>
func (t T) DropPeerNode(peerNode string) error {
	err := make(chan error)
	op := opDropPeerNode{
		err:  err,
		node: peerNode,
	}
	t.cmdC <- op
	return <-err
}

// GetNodeData returns a deep copy of cluster.Node.<node>
func (t T) GetNodeData(node string) *cluster.NodeData {
	result := make(chan *cluster.NodeData)
	op := opGetNodeData{
		result: result,
		node:   node,
	}
	t.cmdC <- op
	return <-result
}

func (o opDropPeerNode) call(ctx context.Context, d *data) {
	d.counterCmd <- idDropPeerNode
	peerNode := o.node
	// TODO publish event for b2.1 "forget_peer" hook
	delete(d.pending.Cluster.Node[d.localNode].Status.Gen, peerNode)
	hbcache.DropPeer(peerNode)
	if _, ok := d.pending.Cluster.Node[peerNode]; ok {
		d.log.Info().Msgf("evict from cluster node stale peer %s", peerNode)
		delete(d.pending.Cluster.Node, peerNode)
		delete(d.hbGens, peerNode)
		delete(d.subHbMode, peerNode)
		delete(d.hbPatchMsgUpdated, peerNode)
	}
	patch := make(jsondelta.Patch, 0)
	op := jsondelta.Operation{
		OpPath: jsondelta.OperationPath{"cluster", "node", peerNode},
		OpKind: "remove",
	}
	patch = append(patch, op)
	eventId++
	if eventB, err := json.Marshal(patch); err != nil {
		d.log.Error().Err(err).Msg("opSetHeartbeatPing Marshal")
	} else {
		d.bus.Pub(
			msgbus.DataUpdated{RawMessage: eventB},
			labelLocalNode,
		)
	}
	d.bus.Pub(msgbus.ForgetPeer{Node: peerNode}, pubsub.Label{"node", peerNode})
	o.err <- nil
}

func (o opGetNodeData) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetNodeData
	if nodeData, ok := d.pending.Cluster.Node[o.node]; ok {
		o.result <- nodeData.DeepCopy()
	} else {
		o.result <- nil
	}
}
