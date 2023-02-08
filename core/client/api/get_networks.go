package api

import (
	"github.com/opensvc/om3/core/client/request"
)

// GetNetworks describes the daemon networks api handler options.
type GetNetworks struct {
	Base
	Server string `json:"server"`
	Name   string `json:"name"`
}

// NewGetNetworks allocates a DaemonNetworksCmdConfig struct and sets
// default values to its keys.
func NewGetNetworks(t Getter) *GetNetworks {
	r := &GetNetworks{
		Server: "",
	}
	r.SetClient(t)
	r.SetAction("networks")
	r.SetMethod("GET")
	return r
}

func (t *GetNetworks) SetName(name string) *GetNetworks {
	t.Name = name
	return t
}

// Do fetchs the daemon statistics structure from the agent api
func (t GetNetworks) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
