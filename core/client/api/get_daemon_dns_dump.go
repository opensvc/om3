package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// GetDaemonDNSDump describes the daemon statistics api handler options.
type GetDaemonDNSDump struct {
	Base
}

// NewGetDaemonDNSDump allocates a DaemonDNSDumpCmdConfig struct and sets
// default values to its keys.
func NewGetDaemonDNSDump(t Getter) *GetDaemonDNSDump {
	r := &GetDaemonDNSDump{}
	r.SetClient(t)
	r.SetAction("daemon/dns/dump")
	r.SetMethod("GET")
	return r
}

// Do fetchs the daemon dns dump structure from the agent api
func (t GetDaemonDNSDump) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
