package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// GetObjectStatus describes the daemon object selector expression
// resolver options.
type GetObjectStatus struct {
	client         Getter `json:"-"`
	ObjectSelector string `json:"selector"`
}

// NewGetObjectStatus allocates a GetObjectStatus struct and sets
// default values to its keys.
func NewGetObjectStatus(t Getter) *GetObjectStatus {
	return &GetObjectStatus{
		client: t,
	}
}

// Do fetchs the daemon statistics structure from the agent api
func (o GetObjectStatus) Do() ([]byte, error) {
	req := request.NewFor("object_status", o)
	return o.client.Get(*req)
}
