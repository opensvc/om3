package daemondata

import (
	"context"
	"encoding/json"
	"time"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
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
		delete(d.mergedOnPeer, peerNode)
		delete(d.mergedFromPeer, peerNode)
		delete(d.remotesNeedFull, peerNode)
		if _, ok := d.pending.Cluster.Node[peerNode]; ok {
			d.log.Info().Msgf("evict from cluster node stale peer %s", peerNode)
			delete(d.pending.Cluster.Node, peerNode)
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
			msgbus.PubEvent(d.bus, event.Event{
				Kind: "patch",
				ID:   eventId,
				Time: time.Now(),
				Data: eventB,
			})
		}
	}
	msgbus.PubHbNodePing(d.bus, peerNode, msgbus.HbNodePing{
		Node:   peerNode,
		Status: o.ping,
	})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
