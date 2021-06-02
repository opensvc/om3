package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// GetNodesInfo describes the options supported by GET /nodes
type GetNodesInfo struct {
	Base
}

// NewGetNodesInfo allocates a GetNodesInfo struct and sets
// default values to its keys.
func NewGetNodesInfo(t Getter) *GetNodesInfo {
	r := &GetNodesInfo{}
	r.SetClient(t)
	r.SetAction("nodes")
	r.SetMethod("GET")
	return r
}

// Do returns the decoded value of an object key
func (t GetNodesInfo) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
