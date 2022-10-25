package daemondata

import (
	"context"
	"encoding/json"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opSetHeartbeats struct {
		err   chan<- error
		value []cluster.HeartbeatThreadStatus
	}
)

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
		msgbus.PubEvent(d.bus, event.Event{
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
