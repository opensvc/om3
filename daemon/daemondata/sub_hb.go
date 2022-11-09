package daemondata

import (
	"context"
	"encoding/json"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opSetSubHb struct {
		err   chan<- error
		value cluster.SubHb
	}
)

// SetSubHb sets sub.heartbeats
func SetSubHb(c chan<- interface{}, subHb cluster.SubHb) error {
	err := make(chan error)
	op := opSetSubHb{
		err:   err,
		value: subHb,
	}
	c <- op
	return <-err
}

func (o opSetSubHb) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetSubHb
	d.pending.Sub.Hb = o.value
	// TODO Use a dedicated msg for heartbeats updates
	eventId++
	patch := make(jsondelta.Patch, 0)
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"sub", "hb"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	patch = append(patch, op)
	if eventB, err := json.Marshal(patch); err != nil {
		d.log.Error().Err(err).Msg("opSetSubHb Marshal")
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
