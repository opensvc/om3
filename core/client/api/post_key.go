package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// PostKey describes the options supported by POST /key
type PostKey struct {
	Base
	Path string `json:"path"`
	Key  string `json:"key"`
	Data []byte `json:"data"`
}

// NewPostKey allocates a PostKey struct and sets
// default values to its keys.
func NewPostKey(t Poster) *PostKey {
	r := &PostKey{}
	r.SetClient(t)
	r.SetAction("key")
	r.SetMethod("POST")
	return r
}

// Do returns the decoded value of an object key
func (t PostKey) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
