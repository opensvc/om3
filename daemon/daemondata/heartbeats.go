package daemondata

import (
	"context"
	"encoding/json"

	"opensvc.com/opensvc/daemon/hbcache"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	opSetHeartbeatPing struct {
		err      chan<- error
		peerNode string
		ping     bool
	}
)

// SetHeartbeatPing update cluster.node.
func SetHeartbeatPing(c chan<- interface{}, peerNode string, ping bool) error {
	err := make(chan error)
	op := opSetHeartbeatPing{
		err:      err,
		peerNode: peerNode,
		ping:     ping,
	}
	c <- op
	return <-err
}

func (o opSetHeartbeatPing) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetHeartbeatPing
	peerNode := o.peerNode
	if !o.ping {
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
	}
	d.bus.Pub(
		msgbus.HbNodePing{
			Node:   peerNode,
			Status: o.ping,
		},
		pubsub.Label{"node", peerNode},
	)
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
