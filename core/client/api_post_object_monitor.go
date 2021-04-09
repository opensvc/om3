package client

import "opensvc.com/opensvc/core/client/request"

// PostObjectMonitor describes the daemon object selector expression
// resolver options.
type PostObjectMonitor struct {
	client         *T     `json:"-"`
	ObjectSelector string `json:"path"`
	GlobalExpect   string `json:"global_expect"`
}

// NewPostObjectMonitor allocates a PostObjectMonitor struct and sets
// default values to its keys.
func (t *T) NewPostObjectMonitor() *PostObjectMonitor {
	return &PostObjectMonitor{
		client: t,
	}
}

// Do ...
func (o PostObjectMonitor) Do() ([]byte, error) {
	req := request.New()
	req.Action = "object_monitor"
	req.Options["path"] = o.ObjectSelector
	req.Options["global_expect"] = o.GlobalExpect
	return o.client.Post(*req)
}
