package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// PostObjectMonitor describes the daemon object selector expression
// resolver options.
type PostObjectMonitor struct {
	client         Poster `json:"-"`
	ObjectSelector string `json:"path"`
	State          string `json:"state,omitempty"`
	LocalExpect    string `json:"local_expect,omitempty"`
	GlobalExpect   string `json:"global_expect,omitempty"`
}

// NewPostObjectMonitor allocates a PostObjectMonitor struct and sets
// default values to its keys.
func NewPostObjectMonitor(t Poster) *PostObjectMonitor {
	return &PostObjectMonitor{
		client: t,
	}
}

// Do ...
func (o PostObjectMonitor) Do() ([]byte, error) {
	req := request.NewFor("object_monitor", o)
	return o.client.Post(*req)
}
