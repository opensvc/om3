package daemondata

import (
	"context"

	"github.com/goccy/go-json"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/hbcache"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	opDropPeerNode struct {
		errC
		node string
	}

	opGetNode struct {
		errC
		node   string
		result chan<- *node.Node
	}
)

// DropPeerNode drops cluster.node.<peer>
func (t T) DropPeerNode(peerNode string) error {
	err := make(chan error, 1)
	op := opDropPeerNode{
		errC: err,
		node: peerNode,
	}
	t.cmdC <- op
	return <-err
}

// GetNode returns a deep copy of cluster.Node.<node>
func (t T) GetNode(nodename string) *node.Node {
	err := make(chan error, 1)
	result := make(chan *node.Node, 1)
	op := opGetNode{
		errC:   err,
		result: result,
		node:   nodename,
	}
	t.cmdC <- op
	if <-err != nil {
		return nil
	}
	return <-result
}

func (o opDropPeerNode) call(ctx context.Context, d *data) error {
	d.counterCmd <- idDropPeerNode
	peerNode := o.node
	// TODO publish event for b2.1 "forget_peer" hook
	delete(d.pending.Cluster.Node[d.localNode].Status.Gen, peerNode)
	hbcache.DropPeer(peerNode)
	if _, ok := d.pending.Cluster.Node[peerNode]; ok {
		d.log.Info().Msgf("evict from cluster node stale peer %s", peerNode)
		delete(d.pending.Cluster.Node, peerNode)
		delete(d.hbGens, peerNode)
		delete(d.hbPatchMsgUpdated, peerNode)
		delete(d.hbMsgMode, peerNode)
		delete(d.hbMsgType, peerNode)
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
	return nil
}

func (o opGetNode) call(ctx context.Context, d *data) error {
	d.counterCmd <- idGetNode
	if nodeData, ok := d.pending.Cluster.Node[o.node]; ok {
		o.result <- nodeData.DeepCopy()
	} else {
		o.result <- nil
	}
	return nil
}
