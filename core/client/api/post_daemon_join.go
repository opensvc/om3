package api

import (
	"github.com/opensvc/om3/core/client/request"
)

type PostDaemonJoin struct {
	Base
	Node string
}

// PostDaemonJoin allocates a PostDaemonJoin struct and sets
// default values to its keys.
func NewPostDaemonJoin(t Poster) *PostDaemonJoin {
	r := &PostDaemonJoin{}
	r.SetClient(t)
	r.SetAction("daemon/join")
	r.SetMethod("POST")
	return r
}

// Do ...
func (t PostDaemonJoin) Do() ([]byte, error) {
	req := request.NewFor(t)
	req.Values.Add("node", req.Node)
	return Route(t.client, *req)
}
