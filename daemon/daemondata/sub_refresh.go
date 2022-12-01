package daemondata

import (
	"context"
)

type (
	opSubRefresh struct {
		err chan<- error
	}
)

// SubRefresh update sub...
func SubRefresh(c chan<- any) error {
	err := make(chan error)
	op := opSubRefresh{
		err: err,
	}
	c <- op
	return <-err
}

func (o opSubRefresh) call(ctx context.Context, d *data) {
	d.log.Debug().Msg("refresh daemon data sub...")
	d.setSubHb()
	o.err <- nil
}
