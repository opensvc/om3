package client

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
	opts := NewRequest()
	opts.Action = "object_selector"
	opts.Options["selector"] = o.ObjectSelector
	return o.client.Get(*opts)
}
