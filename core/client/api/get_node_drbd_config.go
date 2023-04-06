package api

import (
	"github.com/opensvc/om3/core/client/request"
	"github.com/opensvc/om3/daemon/api"
)

// GetNodeDrbdConfig is the options supported by the handler.
type GetNodeDrbdConfig struct {
	Base
	api.GetNodeDrbdConfigParams
}

// NewGetNodeDrbdConfig allocates a GetNodeDrbdConfig struct and sets
// default values to its keys.
func NewGetNodeDrbdConfig(t Getter) *GetNodeDrbdConfig {
	r := &GetNodeDrbdConfig{}
	r.SetClient(t)
	r.SetNode("ANY")
	r.SetAction("node/drbd/config")
	r.SetMethod("GET")
	return r
}

// Do submits the request.
func (t GetNodeDrbdConfig) Do() ([]byte, error) {
	req := request.NewFor(t)
	req.Values.Set("name", t.Name)
	return Route(t.client, *req)
}
