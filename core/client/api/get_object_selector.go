package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// GetObjectSelector describes the daemon object selector expression
// resolver options.
type GetObjectSelector struct {
	client         Getter `json:"-"`
	ObjectSelector string `json:"selector"`
}

// NewGetObjectSelector allocates a GetObjectSelector struct and sets
// default values to its keys.
func NewGetObjectSelector(t Getter) *GetObjectSelector {
	return &GetObjectSelector{
		client:         t,
		ObjectSelector: "**",
	}
}

// Do fetchs the daemon statistics structure from the agent api
func (o GetObjectSelector) Do() ([]byte, error) {
	req := request.NewFor("object_selector", o)
	return o.client.Get(*req)
}
