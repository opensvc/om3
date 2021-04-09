package client

import "opensvc.com/opensvc/core/client/request"

// GetObjectSelector describes the daemon object selector expression
// resolver options.
type GetObjectSelector struct {
	client         *T     `json:"-"`
	ObjectSelector string `json:"selector"`
}

// NewGetObjectSelector allocates a GetObjectSelector struct and sets
// default values to its keys.
func (t *T) NewGetObjectSelector() *GetObjectSelector {
	return &GetObjectSelector{
		client:         t,
		ObjectSelector: "**",
	}
}

// Do fetchs the daemon statistics structure from the agent api
func (o GetObjectSelector) Do() ([]byte, error) {
	req := request.New()
	req.Action = "object_selector"
	req.Options["selector"] = o.ObjectSelector
	return o.client.Get(*req)
}
