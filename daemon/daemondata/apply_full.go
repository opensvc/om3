package daemondata

import (
	"encoding/json"

	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
)

func (d *data) applyFull(msg *hbtype.Msg) error {
	d.counterCmd <- idApplyFull
	remote := msg.Nodename
	local := d.localNode
	d.log.Debug().Msgf("applyFull %s", remote)

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
		d.bus.Pub(msgbus.DataUpdated{RawMessage: eventB}, labelLocalNode)
	}
	return nil
}
