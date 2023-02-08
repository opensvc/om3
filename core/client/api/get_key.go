package api

import (
	"github.com/opensvc/om3/core/client/request"
)

// GetKey describes the options supported by GET /key
type GetKey struct {
	Base
	Path string `json:"path"`
	Key  string `json:"key"`
}

// NewGetKey allocates a GetKey struct and sets
// default values to its keys.
func NewGetKey(t Getter) *GetKey {
	r := &GetKey{}
	r.SetClient(t)
	r.SetAction("key")
	r.SetMethod("GET")
	return r
}

// Do returns the decoded value of an object key
func (t GetKey) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
