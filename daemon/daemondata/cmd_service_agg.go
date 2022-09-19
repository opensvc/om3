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
		srcEv *msgbus.Msg
	}
)

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
			var data json.RawMessage = eventB
			msgbus.PubEvent(d.bus, event.Event{
				Kind: "patch",
				ID:   eventId,
				Time: time.Now(),
				Data: &data,
			})
		}
	}
	msgbus.PubSvcAggDelete(d.bus, s, msgbus.MonSvcAggDeleted{
		Path: o.path,
		Node: d.localNode,
	})
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
		var data json.RawMessage = eventB
		msgbus.PubEvent(d.bus, event.Event{
			Kind: "patch",
			ID:   eventId,
			Time: time.Now(),
			Data: &data,
		})
	}
	msgbus.PubSvcAggUpdate(d.bus, s, msgbus.MonSvcAggUpdated{
		Path:   o.path,
		Node:   d.localNode,
		SvcAgg: o.value,
		SrcEv:  o.srcEv,
	})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
