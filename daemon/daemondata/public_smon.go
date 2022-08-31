package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
)

// DelSmon
//
// committed.Monitor.Node.<localhost>.services.smon.*
func DelSmon(ctx context.Context, c chan<- interface{}, p path.T) error {
	err := make(chan error)
	op := opDelSmon{
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

// SetSmon
//
// committed.Monitor.Node.<localhost>.services.smon.*
func SetSmon(ctx context.Context, c chan<- interface{}, p path.T, v instance.Monitor) error {
	err := make(chan error)
	op := opSetSmon{
		err:   err,
		path:  p,
		value: v,
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
