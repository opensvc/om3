package client

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
	opts := NewRequest()
	opts.Action = "object_action"
	opts.Options["path"] = o.ObjectSelector
	opts.Options["node"] = o.NodeSelector
	opts.Options["action"] = o.Action
	opts.Options["options"] = o.Options
	return o.client.Post(*opts)
}
