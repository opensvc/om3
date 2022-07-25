package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
)

// DelInstanceConfig
//
// committed.Monitor.Node.*.services.config.*
func DelInstanceConfig(ctx context.Context, c chan<- interface{}, p path.T) error {
	err := make(chan error)
	op := opDelInstanceConfig{
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

// SetInstanceConfig
//
// committed.Monitor.Node.*.services.config.*
func SetInstanceConfig(ctx context.Context, c chan<- interface{}, p path.T, v instance.Config) error {
	err := make(chan error)
	op := opSetInstanceConfig{
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
