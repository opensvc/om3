package client

import "opensvc.com/opensvc/core/client/request"

// PostNodeAction describes the daemon object selector expression
// resolver options.
type PostNodeAction struct {
	client       *T                     `json:"-"`
	NodeSelector string                 `json:"node"`
	Action       string                 `json:"action"`
	Options      map[string]interface{} `json:"options"`
}

// NewPostNodeAction allocates a PostNodeAction struct and sets
// default values to its keys.
func (t *T) NewPostNodeAction() *PostNodeAction {
	return &PostNodeAction{
		client:  t,
		Options: make(map[string]interface{}),
	}
}

// Do fetchs the daemon statistics structure from the agent api
func (o PostNodeAction) Do() ([]byte, error) {
	req := request.New()
	req.Action = "node_action"
	req.Options["node"] = o.NodeSelector
	req.Options["action"] = o.Action
	req.Options["options"] = o.Options
	return o.client.Post(*req)
}
