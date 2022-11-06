package daemondata

import (
	"context"
	"encoding/json"
	"time"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	opDelServiceAgg struct {
		err  chan<- error
		path path.T
	}

	opSetServiceAgg struct {
		err   chan<- error
		path  path.T
		value object.AggregatedStatus
		srcEv any
	}
)

// DelServiceAgg
//
// cluster.object.*
func DelServiceAgg(c chan<- interface{}, p path.T) error {
	err := make(chan error)
	op := opDelServiceAgg{
		err:  err,
		path: p,
	}
	c <- op
	return <-err
}

// SetServiceAgg
//
// cluster.object.*
func SetServiceAgg(c chan<- interface{}, p path.T, v object.AggregatedStatus, ev any) error {
	err := make(chan error)
	op := opSetServiceAgg{
		err:   err,
		path:  p,
		value: v,
		srcEv: ev,
	}
	c <- op
	return <-err
}

func (o opDelServiceAgg) setError(err error) {
	o.err <- err
}

func (o opSetServiceAgg) setError(err error) {
	o.err <- err
}

func (o opDelServiceAgg) call(ctx context.Context, d *data) {
	d.counterCmd <- idDelServiceAgg
	s := o.path.String()
	if _, ok := d.pending.Cluster.Object[s]; ok {
		delete(d.pending.Cluster.Object, s)
		patch := jsondelta.Patch{jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"cluster", "object", s},
			OpKind: "remove",
		}}
		if eventB, err := json.Marshal(patch); err != nil {
			d.log.Error().Err(err).Msg("eventCommitPendingOps Marshal fromRootPatch")
		} else {
			eventId++
			msgbus.Pub(d.bus, event.Event{
				Kind: "patch",
				ID:   eventId,
				Time: time.Now(),
				Data: eventB,
			})
		}
	}
	msgbus.Pub(d.bus, msgbus.ObjectAggDeleted{
		Path: o.path,
		Node: d.localNode,
	}, pubsub.Label{"path", s})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}

func (o opSetServiceAgg) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetServiceAgg
	s := o.path.String()
	d.pending.Cluster.Object[s] = o.value

	patch := jsondelta.Patch{jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"cluster", "object", s},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}}
	if eventB, err := json.Marshal(patch); err != nil {
		d.log.Error().Err(err).Msg("eventCommitPendingOps Marshal fromRootPatch")
	} else {
		eventId++
		msgbus.Pub(d.bus, event.Event{
			Kind: "patch",
			ID:   eventId,
			Time: time.Now(),
			Data: eventB,
		})
	}
	msgbus.Pub(d.bus, msgbus.ObjectAggUpdated{
		Path:             o.path,
		Node:             d.localNode,
		AggregatedStatus: o.value,
		SrcEv:            o.srcEv,
	}, pubsub.Label{"path", s})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
