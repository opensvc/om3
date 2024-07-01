package daemondata

import (
	"context"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/dsubsystem"
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

// DropPeerNode is a public method to drop peer node from t. It uses private call
// to func (d *data) dropPeer.
// It is used by a node stale detector to call drop peer on stale <peer> detection.
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

// dropPeer handle actions needed when <peer> node is dropped
//
// It drops <peer> from hbcache
// It drops <peer> from instance data holders and publish associated msgbus.Instance<xxx>Deleted
// It drops <peer> node data holder and publish associated msgbus.Node<xxx>Deleted
// It delete <peer> d.clusterData.Cluster.Node
// It calls setDaemonHb()
// It publish ForgetPeer
func (d *data) dropPeer(peer string) {
	d.log.Infof("drop peer node %s", peer)
	peerLabels := []pubsub.Label{{"node", peer}, {"from", "peer"}}

	hbcache.DropPeer(peer)

	// unset and publish deleted <peer> components instance and node (found from
	// instance and node data holders).
	d.log.Infof("unset and publish deleted peer %s components", peer)
	for p := range instance.ConfigData.GetByNode(peer) {
		instance.ConfigData.Unset(p, peer)
		d.bus.Pub(&msgbus.InstanceConfigDeleted{Node: peer, Path: p}, append(peerLabels, pubsub.Label{"path", p.String()})...)
	}
	for p := range instance.StatusData.GetByNode(peer) {
		instance.StatusData.Unset(p, peer)
		d.bus.Pub(&msgbus.InstanceStatusDeleted{Node: peer, Path: p}, append(peerLabels, pubsub.Label{"path", p.String()})...)
	}
	for p := range instance.MonitorData.GetByNode(peer) {
		instance.MonitorData.Unset(p, peer)
		d.bus.Pub(&msgbus.InstanceMonitorDeleted{Node: peer, Path: p}, append(peerLabels, pubsub.Label{"path", p.String()})...)
	}
	if v := node.MonitorData.Get(peer); v != nil {
		node.DropNode(peer)
		dsubsystem.DropNode(peer)
		// TODO: find a way to clear parts of cluster.node.<peer>.Status
		// TODO: move LsnrData to daemonsubsystem.Listener
		node.LsnrData.Unset(peer) // keep this even if it is already done during DropNode
		d.bus.Pub(&msgbus.ListenerUpdated{Node: peer}, peerLabels...)
		d.bus.Pub(&msgbus.NodeMonitorDeleted{Node: peer}, peerLabels...)
	}

	// delete peer from internal caches
	delete(d.hbGens, peer)
	delete(d.hbGens[d.localNode], peer)
	delete(d.hbPatchMsgUpdated, peer)
	delete(d.hbMsgPatchLength, peer)
	delete(d.hbMsgType, peer)
	delete(d.previousRemoteInfo, peer)

	// delete peer d.clusterData.Cluster.Node...
	if d.clusterData.Cluster.Node[d.localNode].Status.Gen != nil {
		delete(d.clusterData.Cluster.Node[d.localNode].Status.Gen, peer)
	}
	delete(d.clusterData.Cluster.Node, peer)

	d.setDaemonHb()
	d.bus.Pub(&msgbus.ForgetPeer{Node: peer}, peerLabels...)
}
