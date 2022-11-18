package daemondata

import (
	"encoding/json"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/daemon/hbcache"
	"opensvc.com/opensvc/util/jsondelta"
)

func (d *data) setSubHb() {
	d.counterCmd <- idSetSubHb
	subHb := cluster.SubHb{
		Heartbeats: hbcache.Heartbeats(),
		Modes:      hbcache.Modes(),
	}
	d.pending.Sub.Hb = subHb
	// TODO Use a dedicated msg for heartbeats updates
	eventId++
	patch := make(jsondelta.Patch, 0)
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"sub", "hb"},
		OpValue: jsondelta.NewOptValue(subHb),
		OpKind:  "replace",
	}
	patch = append(patch, op)
	if eventB, err := json.Marshal(patch); err != nil {
		d.log.Error().Err(err).Msg("setSubHb Marshal")
	} else {
		d.bus.Pub(event.Event{
			Kind: "patch",
			ID:   eventId,
			Time: time.Now(),
			Data: eventB,
		})
	}
}
