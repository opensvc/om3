package client

import (
	"encoding/json"
	"fmt"
)

// PostNodeAction describes the daemon object selector expression
// resolver options.
type PostNodeAction struct {
	API          API                    `json:"-"`
	NodeSelector string                 `json:"node"`
	Action       string                 `json:"action"`
	Options      map[string]interface{} `json:"options"`
}

// NewPostNodeAction allocates a PostNodeAction struct and sets
// default values to its keys.
func (a API) NewPostNodeAction() *PostNodeAction {
	return &PostNodeAction{
		API:     a,
		Options: make(map[string]interface{}),
	}
}

// Do fetchs the daemon statistics structure from the agent api
func (o PostNodeAction) Do() ([]byte, error) {
	opts := o.API.NewRequest()
	opts.Action = "object_action"
	opts.Options["node"] = o.NodeSelector
	opts.Options["action"] = o.Action
	opts.Options["options"] = o.Options
	b, _ := json.Marshal(opts)
	fmt.Println(string(b))
	return o.API.Requester.Post(*opts)
}
