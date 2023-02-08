package api

import (
	"github.com/opensvc/om3/core/client/request"
)

// PostObjectProgress describes the daemon object selector expression
// resolver options.
type PostObjectProgress struct {
	Base
	Path      string `json:"path"`
	State     string `json:"state"`
	SessionId string `json:"session_id"`
	IsPartial bool   `json:"is_partial"`
}

// NewPostObjectProgress allocates a PostObjectProgress struct and sets
// default values to its keys.
func NewPostObjectProgress(t Poster) *PostObjectProgress {
	r := &PostObjectProgress{}
	r.SetClient(t)
	r.SetAction("object/progress")
	r.SetMethod("POST")
	return r
}

// Do ...
func (t PostObjectProgress) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
