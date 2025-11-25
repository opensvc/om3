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
func (t T) DaemonRefresh() {
	errC := make(chan error, 1)
	op := opDaemonRefresh{
		errC: errC,
	}
	t.cmdC <- op
	<-errC
}

func (o opDaemonRefresh) call(ctx context.Context, d *data) error {
	d.log.Tracef("refresh daemon data sub...")
	d.setDaemonHeartbeat()
	return nil
}
