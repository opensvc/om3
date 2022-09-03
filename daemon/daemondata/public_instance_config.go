package daemondata

import (
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
)

// DelInstanceConfig
//
// committed.Monitor.Node.*.services.config.*
func DelInstanceConfig(c chan<- interface{}, p path.T) error {
	err := make(chan error)
	op := opDelInstanceConfig{
		err:  err,
		path: p,
	}
	c <- op
	return <-err
}

// SetInstanceConfig
//
// committed.Monitor.Node.*.services.config.*
func SetInstanceConfig(dataCmdC chan<- interface{}, p path.T, v instance.Config) error {
	err := make(chan error)
	op := opSetInstanceConfig{
		err:   err,
		path:  p,
		value: v,
	}
	dataCmdC <- op
	return <-err
}
