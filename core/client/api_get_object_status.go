package client

// GetObjectStatus describes the daemon object selector expression
// resolver options.
type GetObjectStatus struct {
	API            API    `json:"-"`
	ObjectSelector string `json:"selector"`
}

// NewGetObjectStatus allocates a GetObjectStatus struct and sets
// default values to its keys.
func (a API) NewGetObjectStatus() *GetObjectStatus {
	return &GetObjectStatus{
		API: a,
	}
}

// Do fetchs the daemon statistics structure from the agent api
func (o GetObjectStatus) Do() ([]byte, error) {
	opts := NewRequest()
	opts.Action = "object_status"
	opts.Options["selector"] = o.ObjectSelector
	return o.API.Requester.Get(*opts)
}
