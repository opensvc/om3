package daemondata

import (
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
)

// DelInstanceStatus
//
// committed.Monitor.Node.<localhost>.services.status.*
func DelInstanceStatus(c chan<- interface{}, p path.T) error {
	err := make(chan error)
	op := opDelInstanceStatus{
		err:  err,
		path: p,
	}
	c <- op
	return <-err
}

// GetInstanceStatus
//
// committed.Monitor.Node.<localhost>.services.status.*
func GetInstanceStatus(c chan<- interface{}, p path.T, node string) instance.Status {
	status := make(chan instance.Status)
	op := opGetInstanceStatus{
		status: status,
		path:   p,
		node:   node,
	}
	c <- op
	return <-status
}

// SetInstanceStatus
//
// committed.Monitor.Node.<localhost>.services.status.*
func SetInstanceStatus(c chan<- interface{}, p path.T, v instance.Status) error {
	err := make(chan error)
	op := opSetInstanceStatus{
		err:   err,
		path:  p,
		value: v,
	}
	c <- op
	return <-err
}
