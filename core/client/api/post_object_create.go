package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// PostObjectCreate are options supported by the api handler.
type PostObjectCreate struct {
	Base
	ObjectSelector string                 `json:"path,omitempty"`
	Namespace      string                 `json:"namespace,omitempty"`
	Template       string                 `json:"template,omitempty"`
	Provision      bool                   `json:"provision,omitempty"`
	Restore        bool                   `json:"restore,omitempty"`
	Force          bool                   `json:"force,omitempty"`
	Data           map[string]interface{} `json:"data,omitempty"`
}

// NewPostObjectCreate allocates a PostObjectCreate struct and sets
// default values to its keys.
func NewPostObjectCreate(t Poster) *PostObjectCreate {
	r := &PostObjectCreate{
		Data: make(map[string]interface{}),
	}
	r.SetClient(t)
	r.SetAction("object_create")
	r.SetMethod("POST")
	return r
}

// Do executes the request and returns the undecoded bytes.
func (t PostObjectCreate) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
