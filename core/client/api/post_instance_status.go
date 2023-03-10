package api

import (
	"github.com/opensvc/om3/core/client/request"
	"github.com/opensvc/om3/core/instance"
)

// PostInstanceStatus describes the daemon object selector expression
// resolver options.
type PostInstanceStatus struct {
	Base
	Path string          `json:"path"`
	Data instance.Status `json:"status"`
}

// NewPostInstanceStatus allocates a PostInstanceStatus struct and sets
// default values to its keys.
func NewPostInstanceStatus(t Poster) *PostInstanceStatus {
	r := &PostInstanceStatus{}
	r.SetClient(t)
	r.SetAction("instance/status")
	r.SetMethod("POST")
	return r
}

// Do fetchs the daemon statistics structure from the agent api
func (t PostInstanceStatus) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
