package daemondata

import "opensvc.com/opensvc/core/cluster"

// DelNmon
//
// committed.Monitor.Node.<localhost>.monitor
func DelNmon(c chan<- interface{}) error {
	err := make(chan error)
	op := opDelNmon{
		err: err,
	}
	c <- op
	return <-err
}

// SetNmon
//
// committed.Monitor.Node.<localhost>.monitor
func SetNmon(c chan<- interface{}, v cluster.NodeMonitor) error {
	err := make(chan error)
	op := opSetNmon{
		err:   err,
		value: v,
	}
	c <- op
	return <-err
}
