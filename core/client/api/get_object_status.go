package api

import (
	"github.com/opensvc/om3/core/client/request"
)

// GetObjectStatus describes the daemon object selector expression
// resolver options.
type GetObjectStatus struct {
	Base
	ObjectSelector string `json:"selector"`
}

// NewGetObjectStatus allocates a GetObjectStatus struct and sets
// default values to its keys.
func NewGetObjectStatus(t Getter) *GetObjectStatus {
	r := &GetObjectStatus{}
	r.SetClient(t)
	r.SetAction("object_status")
	r.SetMethod("GET")
	return r
}

// Do fetchs the daemon statistics structure from the agent api
func (t GetObjectStatus) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
