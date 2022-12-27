package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// PostObjectSwitchTo describes the daemon object selector expression
// resolver options.
type PostObjectSwitchTo struct {
	Base
	ObjectSelector string   `json:"path"`
	Destination    []string `json:"destination"`
}

// NewPostObjectSwitchTo allocates a PostObjectSwitchTo struct and sets
// default values to its keys.
func NewPostObjectSwitchTo(t Poster) *PostObjectSwitchTo {
	r := &PostObjectSwitchTo{}
	r.SetClient(t)
	r.SetAction("object/switchTo")
	r.SetMethod("POST")
	return r
}

// Do ...
func (t PostObjectSwitchTo) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
