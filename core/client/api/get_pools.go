package api

import (
	"github.com/opensvc/om3/core/client/request"
)

// GetPools describes the daemon pools api handler options.
type GetPools struct {
	Base
	Server string `json:"server"`
	Name   string `json:"name"`
}

// NewGetPools allocates a DaemonPoolsCmdConfig struct and sets
// default values to its keys.
func NewGetPools(t Getter) *GetPools {
	r := &GetPools{
		Server: "",
	}
	r.SetClient(t)
	r.SetAction("pools")
	r.SetMethod("GET")
	return r
}

func (t *GetPools) SetName(name string) *GetPools {
	t.Name = name
	return t
}

// Do fetchs the daemon statistics structure from the agent api
func (t GetPools) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
