package daemondata

import (
	"context"
)

type (
	opSubRefresh struct {
		err chan<- error
	}
)

// SubRefresh updates the private dataset of a daemon subsystem
// (scheduler, dns, ...)
func (t T) SubRefresh() error {
	err := make(chan error)
	op := opSubRefresh{
		err: err,
	}
	t.cmdC <- op
	return <-err
}

func (o opSubRefresh) call(ctx context.Context, d *data) {
	d.log.Debug().Msg("refresh daemon data sub...")
	d.setSubHb()
	o.err <- nil
}
