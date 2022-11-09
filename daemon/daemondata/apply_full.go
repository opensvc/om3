package daemondata

import (
	"context"
	"encoding/json"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/util/jsondelta"
)

type opApplyRemoteFull struct {
	nodename string
	full     *cluster.NodeData
	done     chan<- bool
}

func (o opApplyRemoteFull) call(ctx context.Context, d *data) {
	d.counterCmd <- idApplyFull
	remote := o.nodename
	local := d.localNode
	d.log.Debug().Msgf("opApplyRemoteFull %s", remote)
	defer func() {
		d.log.Debug().
			Interface("remotesNeedFull", d.remotesNeedFull).
			Interface("mergedOnPeer", d.mergedOnPeer).
			Interface("pending gen", d.pending.Cluster.Node[remote].Status.Gen).
			Interface("full.gen", o.full.Status.Gen).
			Msgf("opApplyRemoteFull %s", remote)
		select {
		case <-ctx.Done():
		case o.done <- true:
		}
	}()

	if d.mergedFromPeer[remote] == o.full.Status.Gen[remote] && d.mergedOnPeer[remote] == o.full.Status.Gen[local] {
		d.log.Debug().Msgf("apply full drop already applied %s", remote)
		return
	}
	if gen, ok := o.full.Status.Gen[local]; ok {
		d.mergedOnPeer[o.nodename] = gen
	}
	d.mergedFromPeer[remote] = o.full.Status.Gen[remote]
	d.pending.Cluster.Node[remote] = *o.full
	d.pending.Cluster.Node[local].Status.Gen[remote] = o.full.Status.Gen[remote]

	if d.remotesNeedFull[remote] {
		d.log.Info().Msgf("apply full for remote %s (reset need full)", remote)
		d.remotesNeedFull[remote] = false
	}

	absolutePatch := jsondelta.Patch{
		jsondelta.Operation{
			OpPath:  jsondelta.OperationPath{"cluster", "node", remote},
			OpValue: jsondelta.NewOptValue(o.full),
			OpKind:  "replace",
		},
	}

	if eventB, err := json.Marshal(absolutePatch); err != nil {
		d.log.Error().Err(err).Msgf("Marshal absolutePatch %s", remote)
	} else {
		eventId++
		d.bus.Pub(event.Event{
			Kind: "patch",
			ID:   eventId,
			Time: time.Now(),
			Data: eventB,
		})
	}
}

func (t T) ApplyFull(nodename string, full *cluster.NodeData) {
	done := make(chan bool)
	t.cmdC <- opApplyRemoteFull{
		nodename: nodename,
		full:     full,
		done:     done,
	}
	<-done
}
