package api

import "opensvc.com/opensvc/core/client/request"

// PostObjectAction describes the daemon object selector expression
// resolver options.
type PostObjectAction struct {
	Base
	ObjectSelector string                 `json:"path"`
	NodeSelector   string                 `json:"node"`
	Action         string                 `json:"action"`
	Options        map[string]interface{} `json:"options"`
}

// NewPostObjectAction allocates a PostObjectAction struct and sets
// default values to its keys.
func NewPostObjectAction(t Poster) *PostObjectAction {
	r := &PostObjectAction{
		Options: make(map[string]interface{}),
	}
	r.SetClient(t)
	r.SetAction("object_action")
	r.SetMethod("POST")
	return r
}

// Do fetchs the daemon statistics structure from the agent api
func (t PostObjectAction) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
