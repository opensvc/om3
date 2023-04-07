package api

import (
	"github.com/opensvc/om3/core/client/request"
)

// GetNodeDrbdAllocation is the options supported by the handler.
type GetNodeDrbdAllocation struct {
	Base
}

// NewGetNodeDrbdAllocation allocates a GetNodeDrbdAllocation struct and sets
// default values to its keys.
func NewGetNodeDrbdAllocation(t Getter) *GetNodeDrbdAllocation {
	r := &GetNodeDrbdAllocation{}
	r.SetClient(t)
	r.SetNode("ANY")
	r.SetAction("node/drbd/allocation")
	r.SetMethod("GET")
	return r
}

// Do submits the request.
func (t GetNodeDrbdAllocation) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
