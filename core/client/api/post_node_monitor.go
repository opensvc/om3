package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// PostNodeMonitor describes the daemon object selector expression
// resolver options.
type PostNodeMonitor struct {
	Base
	GlobalExpect string `json:"global_expect"`
	Status       string `json:"status"`
}

// NewPostNodeMonitor allocates a PostNodeMonitor struct and sets
// default values to its keys.
func NewPostNodeMonitor(t Poster) *PostNodeMonitor {
	r := &PostNodeMonitor{}
	r.SetClient(t)
	r.SetMethod("POST")
	r.SetAction("/node/monitor")
	return r
}

// Do ...
func (t PostNodeMonitor) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
