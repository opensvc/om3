package client

// GetObjectConfig is the options supported by the handler.
type GetObjectConfig struct {
	client         *T     `json:"-"`
	ObjectSelector string `json:"path"`
	Evaluate       bool   `json:"evaluate,omitempty"`
	Impersonate    string `json:"impersonate,omitempty"`
	Format         string `json:"format,omitempty"`
}

// NewGetObjectConfig allocates a GetObjectConfig struct and sets
// default values to its keys.
func (t *T) NewGetObjectConfig() *GetObjectConfig {
	return &GetObjectConfig{
		client: t,
		Format: "json",
	}
}

// Do submits the request.
func (o GetObjectConfig) Do() ([]byte, error) {
	opts := NewRequest()
	opts.Action = "object_config"
	opts.Options["path"] = o.ObjectSelector
	opts.Options["evaluate"] = o.Evaluate
	opts.Options["impersonate"] = o.Impersonate
	opts.Options["format"] = o.Format
	return o.client.Get(*opts)
}
