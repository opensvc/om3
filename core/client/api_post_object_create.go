package client

// PostObjectCreate are options supported by the api handler.
type PostObjectCreate struct {
	client         *T `json:"-"`
	action         string
	ObjectSelector string                 `json:"path,omitempty"`
	Namespace      string                 `json:"namespace,omitempty"`
	Template       string                 `json:"template,omitempty"`
	Provision      bool                   `json:"provision,omitempty"`
	Restore        bool                   `json:"restore,omitempty"`
	Data           map[string]interface{} `json:"data,omitempty"`
}

// NewPostObjectCreate allocates a PostObjectCreate struct and sets
// default values to its keys.
func (t *T) NewPostObjectCreate() *PostObjectCreate {
	return &PostObjectCreate{
		client: t,
		action: "object_create",
		Data:   make(map[string]interface{}),
	}
}

// Do executes the request and returns the undecoded bytes.
func (o PostObjectCreate) Do() ([]byte, error) {
	req := NewRequestFor(o.action, o)
	return o.client.Post(*req)
}
