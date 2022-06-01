package daemondata

import (
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
)

// DelSmon
//
// committed.Monitor.Node.<localhost>.services.smon.*
func DelSmon(c chan<- interface{}, p path.T) error {
	err := make(chan error)
	c <- opDelSmon{
		err:  err,
		path: p,
	}
	return <-err
}

// SetSmon
//
// committed.Monitor.Node.<localhost>.services.smon.*
func SetSmon(c chan<- interface{}, p path.T, v instance.Monitor) error {
	err := make(chan error)
	c <- opSetSmon{
		err:   err,
		path:  p,
		value: v,
	}
	return <-err
}
