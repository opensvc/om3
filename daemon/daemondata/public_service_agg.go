package daemondata

import (
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
)

// DelServiceAgg
//
// committed.Monitor.Services.*
func DelServiceAgg(c chan<- interface{}, p path.T) error {
	err := make(chan error)
	op := opDelServiceAgg{
		err:  err,
		path: p,
	}
	c <- op
	return <-err
}

// SetServiceAgg
//
// committed.Monitor.Services.*
func SetServiceAgg(c chan<- interface{}, p path.T, v object.AggregatedStatus, ev *msgbus.Msg) error {
	err := make(chan error)
	op := opSetServiceAgg{
		err:   err,
		path:  p,
		value: v,
		srcEv: ev,
	}
	c <- op
	return <-err
}
