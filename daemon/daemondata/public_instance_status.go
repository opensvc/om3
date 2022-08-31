package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
)

// DelInstanceStatus
//
// committed.Monitor.Node.<localhost>.services.status.*
func DelInstanceStatus(ctx context.Context, c chan<- interface{}, p path.T) error {
	err := make(chan error)
	op := opDelInstanceStatus{
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

// GetInstanceStatus
//
// committed.Monitor.Node.<localhost>.services.status.*
func GetInstanceStatus(ctx context.Context, c chan<- interface{}, p path.T, node string) instance.Status {
	status := make(chan instance.Status)
	op := opGetInstanceStatus{
		status: status,
		path:   p,
		node:   node,
	}
	select {
	case <-ctx.Done():
		return instance.Status{}
	case c <- op:
		select {
		case <-ctx.Done():
			return instance.Status{}
		case e := <-status:
			return e
		}
	}
}

// SetInstanceStatus
//
// committed.Monitor.Node.<localhost>.services.status.*
func SetInstanceStatus(ctx context.Context, c chan<- interface{}, p path.T, v instance.Status) error {
	err := make(chan error)
	op := opSetInstanceStatus{
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
