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

type opApplyRemoteFull struct {
	nodename string
	full     *cluster.TNodeData
	done     chan<- bool
}

func (o opApplyRemoteFull) call(ctx context.Context, d *data) {
	d.counterCmd <- idApplyFull
	d.log.Debug().Msgf("opApplyRemoteFull %s", o.nodename)
	d.pending.Cluster.Node[o.nodename] = *o.full
	d.mergedFromPeer[o.nodename] = o.full.Gen[o.nodename]
	d.remotesNeedFull[o.nodename] = false
	if gen, ok := d.pending.Cluster.Node[o.nodename].Gen[d.localNode]; ok {
		d.mergedOnPeer[o.nodename] = gen
	}

	absolutePatch := jsondelta.Patch{
		jsondelta.Operation{
			OpPath:  jsondelta.OperationPath{"cluster", "node", o.nodename},
			OpValue: jsondelta.NewOptValue(o.full),
			OpKind:  "replace",
		},
	}

	if eventB, err := json.Marshal(absolutePatch); err != nil {
		d.log.Error().Err(err).Msgf("Marshal absolutePatch %s", o.nodename)
	} else {
		var eventData json.RawMessage = eventB
		eventId++
		msgbus.PubEvent(d.bus, event.Event{
			Kind: "patch",
			ID:   eventId,
			Time: time.Now(),
			Data: &eventData,
		})
	}

	d.log.Debug().
		Interface("remotesNeedFull", d.remotesNeedFull).
		Interface("mergedOnPeer", d.mergedOnPeer).
		Interface("pending gen", d.pending.Cluster.Node[o.nodename].Gen).
		Interface("full.gen", o.full.Gen).
		Msgf("opApplyRemoteFull %s", o.nodename)
	select {
	case <-ctx.Done():
	case o.done <- true:
	}
}

func (t T) ApplyFull(nodename string, full *cluster.TNodeData) {
	done := make(chan bool)
	t.cmdC <- opApplyRemoteFull{
		nodename: nodename,
		full:     full,
		done:     done,
	}
	<-done
}
