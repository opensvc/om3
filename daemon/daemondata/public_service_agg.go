package daemondata

import (
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
)

// DelServiceAgg
//
// committed.Monitor.Services.*
func DelServiceAgg(c chan<- interface{}, p path.T) error {
	err := make(chan error)
	c <- opDelServiceAgg{
		err:  err,
		path: p,
	}
	return <-err
}

// SetServiceAgg
//
// committed.Monitor.Services.*
func SetServiceAgg(c chan<- interface{}, p path.T, v object.AggregatedStatus) error {
	err := make(chan error)
	c <- opSetServiceAgg{
		err:   err,
		path:  p,
		value: v,
	}
	return <-err
}
