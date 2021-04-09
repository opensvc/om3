package client

import "opensvc.com/opensvc/core/client/request"

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
	req := request.New()
	req.Action = "object_config"
	req.Options["path"] = o.ObjectSelector
	req.Options["evaluate"] = o.Evaluate
	req.Options["impersonate"] = o.Impersonate
	req.Options["format"] = o.Format
	return o.client.Get(*req)
}
