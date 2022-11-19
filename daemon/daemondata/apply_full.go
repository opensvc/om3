package daemondata

import (
	"encoding/json"
	"time"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/util/jsondelta"
)

func (d *data) applyFull(msg *hbtype.Msg) error {
	d.counterCmd <- idApplyFull
	remote := msg.Nodename
	local := d.localNode
	d.log.Debug().Msgf("applyFull %s", remote)

	d.subHbMode[remote] = msg.Kind
	d.pending.Cluster.Node[remote] = msg.Full
	d.pending.Cluster.Node[local].Status.Gen[remote] = msg.Full.Status.Gen[remote]

	absolutePatch := jsondelta.Patch{
		jsondelta.Operation{
			OpPath:  jsondelta.OperationPath{"cluster", "node", remote},
			OpValue: jsondelta.NewOptValue(msg.Full),
			OpKind:  "replace",
		},
	}

	if eventB, err := json.Marshal(absolutePatch); err != nil {
		d.log.Error().Err(err).Msgf("Marshal absolutePatch %s", remote)
		return err
	} else {
		eventId++
		d.bus.Pub(event.Event{
			Kind: "patch",
			ID:   eventId,
			Time: time.Now(),
			Data: eventB,
		})
	}
	return nil
}
