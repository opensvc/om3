package client

import "opensvc.com/opensvc/core/client/request"

// PostNodeMonitor describes the daemon object selector expression
// resolver options.
type PostNodeMonitor struct {
	client       *T     `json:"-"`
	GlobalExpect string `json:"global_expect"`
}

// NewPostNodeMonitor allocates a PostNodeMonitor struct and sets
// default values to its keys.
func (t *T) NewPostNodeMonitor() *PostNodeMonitor {
	return &PostNodeMonitor{
		client: t,
	}
}

// Do ...
func (o PostNodeMonitor) Do() ([]byte, error) {
	req := request.New()
	req.Action = "node_monitor"
	req.Options["global_expect"] = o.GlobalExpect
	return o.client.Post(*req)
}
