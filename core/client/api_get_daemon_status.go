package client

// GetDaemonStatus describes the daemon status api handler options.
type GetDaemonStatus struct {
	client         *T     `json:"-"`
	Namespace      string `json:"namespace,omitempty"`
	ObjectSelector string `json:"selector,omitempty"`
}

// NewGetDaemonStatus allocates a DaemonStatusOptions struct and sets
// default values to its keys.
func (t *T) NewGetDaemonStatus() *GetDaemonStatus {
	return &GetDaemonStatus{
		client:         t,
		Namespace:      "",
		ObjectSelector: "*",
	}
}

// Do fetchs the daemon status structure from the agent api
func (o GetDaemonStatus) Do() ([]byte, error) {
	opts := NewRequest()
	opts.Action = "daemon_status"
	opts.Options["namespace"] = o.Namespace
	opts.Options["selector"] = o.ObjectSelector
	return o.client.Get(*opts)
}
