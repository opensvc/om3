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
	c <- opDelInstanceConfig{
		err:  err,
		path: p,
	}
	return <-err
}

// SetInstanceConfig
//
// committed.Monitor.Node.*.services.config.*
func SetInstanceConfig(c chan<- interface{}, p path.T, v instance.Config) error {
	err := make(chan error)
	c <- opSetInstanceConfig{
		err:   err,
		path:  p,
		value: v,
	}
	return <-err
}
