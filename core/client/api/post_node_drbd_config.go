package api

import (
	"github.com/opensvc/om3/core/client/request"
	"github.com/opensvc/om3/daemon/api"
)

// PostNodeDrbdConfig sends a file content of a supported kind, for the
// daemon to write it in a well-known location.
type PostNodeDrbdConfig struct {
	Base
	api.PostNodeDrbdConfigParams
	api.PostNodeDrbdConfigRequestBody
}

func NewPostNodeDrbdConfig(t Poster) *PostNodeDrbdConfig {
	r := &PostNodeDrbdConfig{}
	r.SetClient(t)
	r.SetMethod("POST")
	r.SetAction("/node/drbd/config")
	return r
}

// Do ...
func (t PostNodeDrbdConfig) Do() ([]byte, error) {
	req := request.NewFor(t)
	req.Options["data"] = t.Data
	req.Options["allocation_id"] = t.AllocationId
	req.Values.Add("name", t.Name)
	return Route(t.client, *req)
}
