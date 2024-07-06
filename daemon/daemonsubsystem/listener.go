package daemonsubsystem

import (
	"net"
)

type (
	// Listener defines daemon Listener subsystem state.
	Listener struct {
		Status

		Addr net.IP `json:"addr"`

		Port int `json:"port"`
	}
)

func (c *Listener) DeepCopy() *Listener {
	var addr net.IP
	if c.Addr != nil {
		addr = append(addr, c.Addr...)
	}
	return &Listener{
		Status: c.Status,
		Addr:   addr,
		Port:   c.Port,
	}
}
