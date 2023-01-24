package api

import (
	"opensvc.com/opensvc/core/client/request"
)

type PostDaemonLeave struct {
	Base
	Node string
}

// NewPostDaemonLeave allocates a PostDaemonLeave struct and sets
// default values to its keys.
func NewPostDaemonLeave(t Poster) *PostDaemonLeave {
	r := &PostDaemonLeave{}
	r.SetClient(t)
	r.SetAction("daemon/leave")
	r.SetMethod("POST")
	return r
}

// Do ...
func (t PostDaemonLeave) Do() ([]byte, error) {
	req := request.NewFor(t)
	req.Values.Add("node", req.Node)
	return Route(t.client, *req)
}
