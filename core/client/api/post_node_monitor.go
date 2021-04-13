package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// PostNodeMonitor describes the daemon object selector expression
// resolver options.
type PostNodeMonitor struct {
	client       Poster `json:"-"`
	GlobalExpect string `json:"global_expect"`
}

// NewPostNodeMonitor allocates a PostNodeMonitor struct and sets
// default values to its keys.
func NewPostNodeMonitor(t Poster) *PostNodeMonitor {
	return &PostNodeMonitor{
		client: t,
	}
}

// Do ...
func (o PostNodeMonitor) Do() ([]byte, error) {
	req := request.NewFor("node_monitor", o)
	return o.client.Post(*req)
}
