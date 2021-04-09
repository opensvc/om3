package client

import "opensvc.com/opensvc/core/client/request"

// PostObjectAction describes the daemon object selector expression
// resolver options.
type PostObjectAction struct {
	client         *T                     `json:"-"`
	ObjectSelector string                 `json:"path"`
	NodeSelector   string                 `json:"node"`
	Action         string                 `json:"action"`
	Options        map[string]interface{} `json:"options"`
}

// NewPostObjectAction allocates a PostObjectAction struct and sets
// default values to its keys.
func (t *T) NewPostObjectAction() *PostObjectAction {
	return &PostObjectAction{
		client:  t,
		Options: make(map[string]interface{}),
	}
}

// Do fetchs the daemon statistics structure from the agent api
func (o PostObjectAction) Do() ([]byte, error) {
	req := request.New()
	req.Action = "object_action"
	req.Options["path"] = o.ObjectSelector
	req.Options["node"] = o.NodeSelector
	req.Options["action"] = o.Action
	req.Options["options"] = o.Options
	return o.client.Post(*req)
}
