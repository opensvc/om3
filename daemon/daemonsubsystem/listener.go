package daemonsubsystem

import (
	"fmt"

	"github.com/opensvc/om3/daemon/daemonenv"
)

type (
	// Listener defines daemon Listener subsystem state.
	Listener struct {
		Status

		Addr string `json:"addr"`

		Port string `json:"port"`
	}
)

func (c *Listener) DeepCopy() *Listener {
	d := *c
	return &d
}

func PeerURL(nodename string) string {
	lsnr := DataListener.Get(nodename)
	addr := nodename
	port := fmt.Sprintf("%d", daemonenv.HTTPPort)
	if lsnr.Port != "" {
		port = lsnr.Port
	}
	if lsnr.Addr != "::" && lsnr.Addr != "" && lsnr.Addr != "0.0.0.0" {
		addr = lsnr.Addr
	}
	return daemonenv.HTTPNodeAndPortURL(addr, port)
}
