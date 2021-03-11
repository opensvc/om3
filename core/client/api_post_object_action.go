package client

// PostObjectAction describes the daemon object selector expression
// resolver options.
type PostObjectAction struct {
	API            API                    `json:"-"`
	ObjectSelector string                 `json:"path"`
	NodeSelector   string                 `json:"node"`
	Action         string                 `json:"action"`
	Options        map[string]interface{} `json:"options"`
}

// NewPostObjectAction allocates a PostObjectAction struct and sets
// default values to its keys.
func (a API) NewPostObjectAction() *PostObjectAction {
	return &PostObjectAction{
		API:     a,
		Options: make(map[string]interface{}),
	}
}

// Do fetchs the daemon statistics structure from the agent api
func (o PostObjectAction) Do() ([]byte, error) {
	opts := NewRequest()
	opts.Action = "object_action"
	opts.Options["path"] = o.ObjectSelector
	opts.Options["node"] = o.NodeSelector
	opts.Options["action"] = o.Action
	opts.Options["options"] = o.Options
	return o.API.Post(*opts)
}
