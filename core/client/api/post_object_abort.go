package api

import (
	"opensvc.com/opensvc/core/client/request"
	"opensvc.com/opensvc/core/path"
)

// PostObjectAbort is the client type to request an orchestration abort
// on a selection of objects.
type PostObjectAbort struct {
	Base
	Path path.T `json:"path"`
}

// NewPostObjectAbort allocates a PostObjectAbort struct and sets
// default values to its keys.
func NewPostObjectAbort(t Poster) *PostObjectAbort {
	r := &PostObjectAbort{}
	r.SetClient(t)
	r.SetAction("object/abort")
	r.SetMethod("POST")
	return r
}

// Do ...
func (t PostObjectAbort) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
