package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// PostNodeAction describes the daemon object selector expression
// resolver options.
type PostNodeAction struct {
	client       Poster                 `json:"-"`
	NodeSelector string                 `json:"node"`
	Action       string                 `json:"action"`
	Options      map[string]interface{} `json:"options"`
}

// NewPostNodeAction allocates a PostNodeAction struct and sets
// default values to its keys.
func NewPostNodeAction(t Poster) *PostNodeAction {
	return &PostNodeAction{
		client:  t,
		Options: make(map[string]interface{}),
	}
}

// Do fetchs the daemon statistics structure from the agent api
func (o PostNodeAction) Do() ([]byte, error) {
	req := request.NewFor("node_action", o)
	return o.client.Post(*req)
}
