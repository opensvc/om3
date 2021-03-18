package client

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
	opts := NewRequest()
	opts.Action = "object_monitor"
	opts.Options["path"] = o.ObjectSelector
	opts.Options["global_expect"] = o.GlobalExpect
	return o.client.Post(*opts)
}
