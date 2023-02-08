package api

import (
	"github.com/opensvc/om3/core/client/request"
)

// PostNodeAction describes the daemon object selector expression
// resolver options.
type PostNodeAction struct {
	Base
	NodeSelector string                 `json:"node"`
	Action       string                 `json:"action"`
	Options      map[string]interface{} `json:"options"`
}

// NewPostNodeAction allocates a PostNodeAction struct and sets
// default values to its keys.
func NewPostNodeAction(t Poster) *PostNodeAction {
	r := &PostNodeAction{
		Options: make(map[string]interface{}),
	}
	r.SetClient(t)
	r.SetAction("node_action")
	r.SetMethod("POST")
	return r
}

// Do fetchs the daemon statistics structure from the agent api
func (t PostNodeAction) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
