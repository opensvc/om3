package client

// PostObjectMonitor describes the daemon object selector expression
// resolver options.
type PostObjectMonitor struct {
	API            API    `json:"-"`
	ObjectSelector string `json:"path"`
	GlobalExpect   string `json:"global_expect"`
}

// NewPostObjectMonitor allocates a PostObjectMonitor struct and sets
// default values to its keys.
func (a API) NewPostObjectMonitor() *PostObjectMonitor {
	return &PostObjectMonitor{
		API: a,
	}
}

// Do ...
func (o PostObjectMonitor) Do() ([]byte, error) {
	opts := o.API.NewRequest()
	opts.Action = "object_monitor"
	opts.Options["path"] = o.ObjectSelector
	opts.Options["global_expect"] = o.GlobalExpect
	return o.API.Requester.Post(*opts)
}
