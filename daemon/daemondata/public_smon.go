package daemondata

import (
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
)

// DelSmon
//
// Monitor.Node.<localhost>.services.smon.*
func DelSmon(c chan<- interface{}, p path.T) error {
	err := make(chan error)
	op := opDelSmon{
		err:  err,
		path: p,
	}
	c <- op
	return <-err
}

// SetSmon
//
// Monitor.Node.<localhost>.services.smon.*
func SetSmon(c chan<- interface{}, p path.T, v instance.Monitor) error {
	err := make(chan error)
	op := opSetSmon{
		err:   err,
		path:  p,
		value: v,
	}
	c <- op
	return <-err
}
