package daemondata

import (
	"context"

	"opensvc.com/opensvc/util/jsondelta"
)

// PushOps append ops to T.pendingOps
func (t T) PushOps(ops []jsondelta.Operation) {
	done := make(chan bool)
	t.cmdC <- opPushOps{
		ops:  ops,
		done: done,
	}
	<-done
}

type opPushOps struct {
	ops  []jsondelta.Operation
	done chan<- bool
}

func (o opPushOps) call(ctx context.Context, d *data) {
	d.counterCmd <- idPushOps
	d.log.Debug().Msgf("opPushOps")
	d.pendingOps = append(d.pendingOps, o.ops...)
	select {
	case <-ctx.Done():
	case o.done <- true:
	}
}
