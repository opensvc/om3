package api

import (
	"opensvc.com/opensvc/core/client/request"
	"opensvc.com/opensvc/core/path"
)

// GetKey describes the options supported by GET /key
type GetObjectsBacklog struct {
	Base
	Filters map[string]interface{}
	Paths   path.L
}

// NewGetObjectsBacklog allocates a GetObjectsBacklog struct and sets
// default values to its keys.
func NewGetObjectsBacklog(t Getter) *GetObjectsBacklog {
	r := &GetObjectsBacklog{}
	r.SetClient(t)
	r.SetAction("objects_backlog")
	r.SetMethod("GET")
	return r
}

// Do returns the decoded value of an object key
func (t GetObjectsBacklog) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
