package api

import (
	"opensvc.com/opensvc/core/client/request"
)

type PostNodeClear struct {
	Base
}

// NewPostNodeClear allocates a PostNodeClear struct and sets
// default values to its keys.
func NewPostNodeClear(t Poster) *PostNodeClear {
	r := &PostNodeClear{}
	r.SetClient(t)
	r.SetAction("node/clear")
	r.SetMethod("POST")
	return r
}

// Do ...
func (t PostNodeClear) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
