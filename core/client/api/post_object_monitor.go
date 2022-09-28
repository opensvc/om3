package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// PostObjectMonitor describes the daemon object selector expression
// resolver options.
type PostObjectMonitor struct {
	Base
	ObjectSelector string `json:"path"`
	State          string `json:"state,omitempty"`
	LocalExpect    string `json:"local_expect,omitempty"`
	GlobalExpect   string `json:"global_expect,omitempty"`
}

// NewPostObjectMonitor allocates a PostObjectMonitor struct and sets
// default values to its keys.
func NewPostObjectMonitor(t Poster) *PostObjectMonitor {
	r := &PostObjectMonitor{}
	r.SetClient(t)
	r.SetAction("object/monitor")
	r.SetMethod("POST")
	return r
}

// Do ...
func (t PostObjectMonitor) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
