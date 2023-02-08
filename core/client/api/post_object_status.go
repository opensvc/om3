package api

import (
	"github.com/opensvc/om3/core/client/request"
	"github.com/opensvc/om3/core/instance"
)

// PostObjectStatus describes the daemon object selector expression
// resolver options.
type PostObjectStatus struct {
	Base
	Path string          `json:"path"`
	Data instance.Status `json:"status"`
}

// NewPostObjectStatus allocates a PostObjectStatus struct and sets
// default values to its keys.
func NewPostObjectStatus(t Poster) *PostObjectStatus {
	r := &PostObjectStatus{}
	r.SetClient(t)
	r.SetAction("object/status")
	r.SetMethod("POST")
	return r
}

// Do fetchs the daemon statistics structure from the agent api
func (t PostObjectStatus) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
