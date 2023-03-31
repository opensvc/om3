package api

import (
	"github.com/opensvc/om3/core/client/request"
)

// GetNodeFile is the options supported by the handler.
type GetNodeFile struct {
	Base
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// NewGetNodeFile allocates a GetNodeFile struct and sets
// default values to its keys.
func NewGetNodeFile(t Getter) *GetNodeFile {
	r := &GetNodeFile{}
	r.SetClient(t)
	r.SetNode("ANY")
	r.SetAction("node/file")
	r.SetMethod("GET")
	return r
}

// Do submits the request.
func (t GetNodeFile) Do() ([]byte, error) {
	req := request.NewFor(t)
	req.Values.Set("kind", t.Kind)
	req.Values.Set("name", t.Name)
	return Route(t.client, *req)
}
