package daemondata

import (
	"context"

	"github.com/opensvc/om3/v3/core/node"
)

type (
	// AckGens is the generation of the data of the local node, and the one
	// each peer says it applied.
	AckGens struct {
		// Local is the generation of the data of the local node.
		Local uint64

		// Pending is true when changes wait for the next commit, which
		// makes a generation Local+1.
		Pending bool

		// MonitorState is the node monitor state of the local node in
		// the data the peers are sent.
		MonitorState node.MonitorState

		// Peers is the generation of the data of the local node each peer
		// applied, as its last message says.
		Peers map[string]uint64
	}

	opGetAckGens struct {
		errC
		result chan<- AckGens
	}
)

// Target returns the generation a peer must have applied to hold the data
// of the local node as it is now, its pending changes included.
func (t AckGens) Target() uint64 {
	if t.Pending {
		return t.Local + 1
	}
	return t.Local
}

// GetAckGens returns the generation of the data of the local node, and the
// one each peer it heard from says it applied: a change of the local node is
// on a peer once the peer applied a generation at least the one of the
// commit holding it.
func (t T) GetAckGens() AckGens {
	result := make(chan AckGens, 1)
	err := make(chan error, 1)
	t.cmdC <- opGetAckGens{errC: err, result: result}
	if <-err != nil {
		return AckGens{}
	}
	return <-result
}

func (o opGetAckGens) call(ctx context.Context, d *data) error {
	gens := AckGens{
		Local:   d.gen,
		Pending: len(d.pendingEvs) > 0,
		Peers:   make(map[string]uint64),
	}
	if localNode, ok := d.clusterData.Cluster.Node[d.localNode]; ok {
		gens.MonitorState = localNode.Monitor.State
	}
	for peer, peerGens := range d.hbGens {
		if peer == d.localNode {
			continue
		}
		gens.Peers[peer] = peerGens[d.localNode]
	}
	o.result <- gens
	return nil
}
