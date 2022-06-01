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
	c <- opDelInstanceStatus{
		err:  err,
		path: p,
	}
	return <-err
}

// GelInstanceStatus
//
// committed.Monitor.Node.<localhost>.services.status.*
func GelInstanceStatus(c chan<- interface{}, p path.T, node string) instance.Status {
	status := make(chan instance.Status)
	c <- opGetInstanceStatus{
		status: status,
		path:   p,
		node:   node,
	}
	return <-status
}

// SetInstanceStatus
//
// committed.Monitor.Node.<localhost>.services.status.*
func SetInstanceStatus(c chan<- interface{}, p path.T, v instance.Status) error {
	err := make(chan error)
	c <- opSetInstanceStatus{
		err:   err,
		path:  p,
		value: v,
	}
	return <-err
}
