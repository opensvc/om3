package daemondata

import (
	"context"
	"encoding/json"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/event"
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

	opSetHeartbeats struct {
		err   chan<- error
		value []cluster.HeartbeatThreadStatus
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

// SetHeartbeats sets sub.heartbeats
func SetHeartbeats(c chan<- interface{}, heartbeats []cluster.HeartbeatThreadStatus) error {
	err := make(chan error)
	hbs := make([]cluster.HeartbeatThreadStatus, 0)
	for _, v := range heartbeats {
		hbs = append(hbs, v)
	}
	op := opSetHeartbeats{
		err:   err,
		value: hbs,
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
			d.bus.Pub(event.Event{
				Kind: "patch",
				ID:   eventId,
				Time: time.Now(),
				Data: eventB,
			})
		}
	}
	d.bus.Pub(msgbus.HbNodePing{
		Node:   peerNode,
		Status: o.ping,
	}, pubsub.Label{"node", peerNode})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}

func (o opSetHeartbeats) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetHeartbeats
	d.pending.Sub.Heartbeats = o.value
	// TODO Use a dedicated msg for heartbeats updates
	eventId++
	patch := make(jsondelta.Patch, 0)
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"sub", "heartbeats"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	patch = append(patch, op)
	if eventB, err := json.Marshal(patch); err != nil {
		d.log.Error().Err(err).Msg("opSetHeartbeats Marshal")
	} else {
		d.bus.Pub(event.Event{
			Kind: "patch",
			ID:   eventId,
			Time: time.Now(),
			Data: eventB,
		})
	}
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
