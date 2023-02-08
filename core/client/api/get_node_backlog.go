package api

import (
	"github.com/opensvc/om3/core/client/request"
)

// GetKey describes the options supported by GET /key
type GetNodeBacklog struct {
	Base
	filters map[string]interface{}
}

// NewGetNodeBacklog allocates a GetNodeBacklog struct and sets
// default values to its keys.
func NewGetNodeBacklog(t Getter) *GetNodeBacklog {
	r := &GetNodeBacklog{}
	r.SetClient(t)
	r.SetAction("node_backlog")
	r.SetMethod("GET")
	return r
}

func (t *GetNodeBacklog) SetFilters(m map[string]interface{}) *GetNodeBacklog {
	t.filters = m
	return t
}

func (t GetNodeBacklog) Filters() map[string]interface{} {
	return t.filters
}

// Do returns the decoded value of an object key
func (t GetNodeBacklog) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
