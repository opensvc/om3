package daemondata

import (
	"context"
)

type (
	opDaemonRefresh struct {
		err chan<- error
	}
)

// DaemonRefresh updates the private dataset of a daemon subsystem
// (scheduler, dns, ...)
func (t T) DaemonRefresh() error {
	err := make(chan error)
	op := opDaemonRefresh{
		err: err,
	}
	t.cmdC <- op
	return <-err
}

func (o opDaemonRefresh) call(ctx context.Context, d *data) {
	d.log.Debug().Msg("refresh daemon data sub...")
	d.setDaemonHb()
	o.err <- nil
}
