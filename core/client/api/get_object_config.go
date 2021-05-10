package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// GetObjectConfig is the options supported by the handler.
type GetObjectConfig struct {
	Base
	ObjectSelector string `json:"path"`
	Evaluate       bool   `json:"evaluate,omitempty"`
	Impersonate    string `json:"impersonate,omitempty"`
	Format         string `json:"format,omitempty"`
}

// NewGetObjectConfig allocates a GetObjectConfig struct and sets
// default values to its keys.
func NewGetObjectConfig(t Getter) *GetObjectConfig {
	r := &GetObjectConfig{
		Format: "json",
	}
	r.SetClient(t)
	r.SetNode("ANY")
	r.SetAction("object_config")
	r.SetMethod("GET")
	return r
}

// Do submits the request.
func (t GetObjectConfig) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
