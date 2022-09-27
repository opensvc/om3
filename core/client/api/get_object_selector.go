package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// GetObjectSelector describes the daemon object selector expression
// resolver options.
type GetObjectSelector struct {
	Base
	ObjectSelector string `json:"-"`
}

// NewGetObjectSelector allocates a GetObjectSelector struct and sets
// default values to its keys.
func NewGetObjectSelector(t Getter) *GetObjectSelector {
	r := &GetObjectSelector{
		ObjectSelector: "**",
	}
	r.SetClient(t)
	r.SetAction("object/selector")
	r.SetMethod("GET")
	return r
}

// Do fetchs the daemon statistics structure from the agent api
func (t GetObjectSelector) Do() ([]byte, error) {
	t.SetQueryArgs(map[string]string{"selector": t.ObjectSelector})
	req := request.NewFor(t)
	return Route(t.client, *req)
}
