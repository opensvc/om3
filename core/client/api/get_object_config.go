package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// GetObjectConfig is the options supported by the handler.
type GetObjectConfig struct {
	client         Getter `json:"-"`
	ObjectSelector string `json:"path"`
	Evaluate       bool   `json:"evaluate,omitempty"`
	Impersonate    string `json:"impersonate,omitempty"`
	Format         string `json:"format,omitempty"`
}

// NewGetObjectConfig allocates a GetObjectConfig struct and sets
// default values to its keys.
func NewGetObjectConfig(t Getter) *GetObjectConfig {
	return &GetObjectConfig{
		client: t,
		Format: "json",
	}
}

// Do submits the request.
func (o GetObjectConfig) Do() ([]byte, error) {
	req := request.NewFor("object_config", o)
	return o.client.Get(*req)
}
