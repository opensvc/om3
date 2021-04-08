package client

// PostObjectCreate are options supported by the api handler.
type PostObjectCreate struct {
	client         *T          `json:"-"`
	ObjectSelector string      `json:"path"`
	Namespace      string      `json:"namespace"`
	Template       string      `json:"template"`
	Provision      bool        `json:"provision"`
	Restore        bool        `json:"restore"`
	Data           interface{} `json:"data"`
}

// NewPostObjectCreate allocates a PostObjectCreate struct and sets
// default values to its keys.
func (t *T) NewPostObjectCreate() *PostObjectCreate {
	return &PostObjectCreate{
		client: t,
	}
}

// Do ...
func (o PostObjectCreate) Do() ([]byte, error) {
	opts := NewRequest()
	opts.Action = "object_create"
	opts.Options["path"] = o.ObjectSelector
	opts.Options["namespace"] = o.Namespace
	opts.Options["provision"] = o.Provision
	opts.Options["template"] = o.Template
	opts.Options["restore"] = o.Restore
	opts.Options["data"] = o.Data
	return o.client.Post(*opts)
}
