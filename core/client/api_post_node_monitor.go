package client

// PostNodeMonitor describes the daemon object selector expression
// resolver options.
type PostNodeMonitor struct {
	API          API    `json:"-"`
	GlobalExpect string `json:"global_expect"`
}

// NewPostNodeMonitor allocates a PostNodeMonitor struct and sets
// default values to its keys.
func (a API) NewPostNodeMonitor() *PostNodeMonitor {
	return &PostNodeMonitor{
		API: a,
	}
}

// Do ...
func (o PostNodeMonitor) Do() ([]byte, error) {
	opts := NewRequest()
	opts.Action = "node_monitor"
	opts.Options["global_expect"] = o.GlobalExpect
	return o.API.Post(*opts)
}
