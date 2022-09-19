package daemondata

import (
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
)

// DelServiceAgg
//
// cluster.object.*
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
// cluster.object.*
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
