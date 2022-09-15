package daemondata

import "opensvc.com/opensvc/core/cluster"

// DelNmon deletes Monitor.Node.<localhost>.monitor
func DelNmon(c chan<- interface{}) error {
	err := make(chan error)
	op := opDelNmon{
		err: err,
	}
	c <- op
	return <-err
}

// SetNmon sets Monitor.Node.<localhost>.monitor
func SetNmon(c chan<- interface{}, v cluster.NodeMonitor) error {
	err := make(chan error)
	op := opSetNmon{
		err:   err,
		value: v,
	}
	c <- op
	return <-err
}

// GetNmon returns Monitor.Node.<node>.monitor
func GetNmon(c chan<- interface{}, node string) cluster.NodeMonitor {
	value := make(chan cluster.NodeMonitor)
	op := opGetNmon{
		value: value,
		node:  node,
	}
	c <- op
	return <-value
}

// GetNodeMonitor returns Monitor.Node.<node>.monitor
func (t T) GetNodeMonitor(node string) cluster.NodeMonitor {
	return GetNmon(t.cmdC, node)
}
