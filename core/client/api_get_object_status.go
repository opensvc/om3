package client

import "opensvc.com/opensvc/core/client/request"

// GetObjectStatus describes the daemon object selector expression
// resolver options.
type GetObjectStatus struct {
	client         *T     `json:"-"`
	ObjectSelector string `json:"selector"`
}

// NewGetObjectStatus allocates a GetObjectStatus struct and sets
// default values to its keys.
func (t *T) NewGetObjectStatus() *GetObjectStatus {
	return &GetObjectStatus{
		client: t,
	}
}

// Do fetchs the daemon statistics structure from the agent api
func (o GetObjectStatus) Do() ([]byte, error) {
	req := request.New()
	req.Action = "object_status"
	req.Options["selector"] = o.ObjectSelector
	return o.client.Get(*req)
}
