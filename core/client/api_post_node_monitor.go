package client

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
	opts := NewRequest()
	opts.Action = "node_monitor"
	opts.Options["global_expect"] = o.GlobalExpect
	return o.client.Post(*opts)
}
