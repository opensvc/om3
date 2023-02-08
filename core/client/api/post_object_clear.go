package api

import (
	"github.com/opensvc/om3/core/client/request"
	"github.com/opensvc/om3/core/path"
)

type PostObjectClear struct {
	Base
	Path path.T `json:"path"`
}

// NewPostObjectClear allocates a PostObjectClear struct and sets
// default values to its keys.
func NewPostObjectClear(t Poster) *PostObjectClear {
	r := &PostObjectClear{}
	r.SetClient(t)
	r.SetAction("object/clear")
	r.SetMethod("POST")
	return r
}

// Do ...
func (t PostObjectClear) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
