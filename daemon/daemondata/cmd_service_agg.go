package daemondata

import (
	"encoding/json"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/jsondelta"
	"opensvc.com/opensvc/util/timestamp"
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
	}
)

func (o opDelServiceAgg) call(d *data) {
	d.counterCmd <- idDelServiceAgg
	s := o.path.String()
	if _, ok := d.pending.Monitor.Services[s]; ok {
		delete(d.pending.Monitor.Services, s)
		patch := jsondelta.Patch{jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"monitor", "services", s},
			OpKind: "remove",
		}}
		if eventB, err := json.Marshal(patch); err != nil {
			d.log.Error().Err(err).Msg("eventCommitPendingOps Marshal fromRootPatch")
		} else {
			eventId++
			var data json.RawMessage = eventB
			daemonps.PubEvent(d.eventCmd, event.Event{
				Kind:      "patch",
				ID:        eventId,
				Timestamp: timestamp.Now(),
				Data:      &data,
			})
		}
	}
	daemonps.PubSvcAggDelete(d.eventCmd, s, moncmd.MonSvcAggDeleted{
		Path: o.path,
		Node: d.localNode,
	})
	o.err <- nil
}

func (o opSetServiceAgg) call(d *data) {
	d.counterCmd <- idSetServiceAgg
	s := o.path.String()
	d.pending.Monitor.Services[s] = o.value

	patch := jsondelta.Patch{jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"monitor", "services", s},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}}
	if eventB, err := json.Marshal(patch); err != nil {
		d.log.Error().Err(err).Msg("eventCommitPendingOps Marshal fromRootPatch")
	} else {
		eventId++
		var data json.RawMessage = eventB
		daemonps.PubEvent(d.eventCmd, event.Event{
			Kind:      "patch",
			ID:        eventId,
			Timestamp: timestamp.Now(),
			Data:      &data,
		})
	}
	daemonps.PubSvcAggUpdate(d.eventCmd, s, moncmd.MonSvcAggUpdated{
		Path:   o.path,
		Node:   d.localNode,
		SvcAgg: o.value,
	})
	o.err <- nil
}
