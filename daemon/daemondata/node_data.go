package daemondata

import (
	"context"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/hbcache"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	opDropPeerNode struct {
		errC
		node string
	}

	opGetClusterNodeData struct {
		errC
		Node string
		Data chan<- *node.Node
	}
)

// DropPeerNode drop peer node
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
	d.dropPeer(o.node)
	return nil
}

// ClusterNodeData returns deep copy of cluster node data for node n.
// It returns nil when node n is not found in cluster data.
func (t T) ClusterNodeData(n string) *node.Node {
	nData := make(chan *node.Node, 1)
	err := make(chan error, 1)
	t.cmdC <- opGetClusterNodeData{
		errC: err,
		Node: n,
		Data: nData,
	}
	if <-err != nil {
		return nil
	}
	return <-nData
}

func (o opGetClusterNodeData) call(ctx context.Context, d *data) error {
	if nData, ok := d.clusterData.Cluster.Node[o.Node]; ok {
		o.Data <- nData.DeepCopy()
	} else {
		o.Data <- nil
	}
	return nil
}

func (d *data) dropPeer(peerNode string) {
	// TODO publish event for b2.1 "forget_peer" hook
	delete(d.clusterData.Cluster.Node[d.localNode].Status.Gen, peerNode)
	hbcache.DropPeer(peerNode)
	if _, ok := d.clusterData.Cluster.Node[peerNode]; ok {
		d.log.Info().Msgf("evict from cluster node stale peer %s", peerNode)
		delete(d.clusterData.Cluster.Node, peerNode)
		delete(d.hbGens, peerNode)
		delete(d.hbGens[d.localNode], peerNode)
		delete(d.hbPatchMsgUpdated, peerNode)
		delete(d.hbMsgPatchLength, peerNode)
		delete(d.hbMsgType, peerNode)
	}
	d.setDaemonHb()
	d.bus.Pub(&msgbus.ForgetPeer{Node: peerNode}, pubsub.Label{"node", peerNode})
}
