package daemondata

import (
	"context"

	"github.com/opensvc/om3/daemon/hbcache"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	opDropPeerNode struct {
		errC
		node string
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

func (o opDropPeerNode) call(ctx context.Context, d *data) error {
	d.statCount[idDropPeerNode]++
	peerNode := o.node
	// TODO publish event for b2.1 "forget_peer" hook
	delete(d.clusterData.Cluster.Node[d.localNode].Status.Gen, peerNode)
	hbcache.DropPeer(peerNode)
	if _, ok := d.clusterData.Cluster.Node[peerNode]; ok {
		d.log.Info().Msgf("evict from cluster node stale peer %s", peerNode)
		delete(d.clusterData.Cluster.Node, peerNode)
		delete(d.hbGens, peerNode)
		delete(d.hbGens[d.localNode], peerNode)
		delete(d.hbPatchMsgUpdated, peerNode)
		delete(d.hbMsgMode, peerNode)
		delete(d.hbMsgType, peerNode)
	}
	d.setDaemonHb()
	d.bus.Pub(&msgbus.ForgetPeer{Node: peerNode}, pubsub.Label{"node", peerNode})
	return nil
}
