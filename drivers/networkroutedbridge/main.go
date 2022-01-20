package networkroutedbridge

import (
	"opensvc.com/opensvc/core/network"
)

type (
	T struct {
		network.T
	}
)

func init() {
	network.Register("routed_bridge", NewNetworker)
}

func NewNetworker() network.Networker {
	t := New()
	var i interface{} = t
	return i.(network.Networker)
}

func New() *T {
	t := T{}
	return &t
}

func (t T) Usage() (network.StatusUsage, error) {
	usage := network.StatusUsage{}
	return usage, nil
}
