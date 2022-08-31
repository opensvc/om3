package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
)

// DelServiceAgg
//
// committed.Monitor.Services.*
func DelServiceAgg(ctx context.Context, c chan<- interface{}, p path.T) error {
	err := make(chan error)
	op := opDelServiceAgg{
		err:  err,
		path: p,
	}
	select {
	case <-ctx.Done():
		return nil
	case c <- op:
		select {
		case <-ctx.Done():
			return nil
		case e := <-err:
			return e
		}
	}
}

// SetServiceAgg
//
// committed.Monitor.Services.*
func SetServiceAgg(ctx context.Context, c chan<- interface{}, p path.T, v object.AggregatedStatus, ev *moncmd.T) error {
	err := make(chan error)
	op := opSetServiceAgg{
		err:   err,
		path:  p,
		value: v,
		srcEv: ev,
	}
	select {
	case <-ctx.Done():
		return nil
	case c <- op:
		select {
		case <-ctx.Done():
			return nil
		case e := <-err:
			return e
		}
	}
}
