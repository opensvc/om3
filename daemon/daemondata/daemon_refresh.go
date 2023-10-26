package daemondata

import (
	"context"
)

type (
	opDaemonRefresh struct {
		errC
	}
)

// DaemonRefresh updates the private dataset of a daemon subsystem
// (scheduler, dns, ...)
func (t T) DaemonRefresh() error {
	err := make(chan error, 1)
	op := opDaemonRefresh{
		errC: err,
	}
	t.cmdC <- op
	return <-err
}

func (o opDaemonRefresh) call(ctx context.Context, d *data) error {
	d.log.Debugf("refresh daemon data sub...")
	d.setDaemonHb()
	return nil
}
