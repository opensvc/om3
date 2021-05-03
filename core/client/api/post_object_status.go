package api

import (
	"opensvc.com/opensvc/core/client/request"
	"opensvc.com/opensvc/core/instance"
)

// PostObjectStatus describes the daemon object selector expression
// resolver options.
type PostObjectStatus struct {
	client Poster          `json:"-"`
	Path   string          `json:"path"`
	Data   instance.Status `json:"data"`
}

// NewPostObjectStatus allocates a PostObjectStatus struct and sets
// default values to its keys.
func NewPostObjectStatus(t Poster) *PostObjectStatus {
	return &PostObjectStatus{
		client: t,
	}
}

// Do fetchs the daemon statistics structure from the agent api
func (o PostObjectStatus) Do() ([]byte, error) {
	req := request.NewFor("object_status", o)
	return o.client.Post(*req)
}
