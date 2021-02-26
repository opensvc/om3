package client

import (
	"encoding/json"

	"opensvc.com/opensvc/core/cluster"
)

// GetDaemonStatus describes the daemon status api handler options.
type GetDaemonStatus struct {
	API            API    `json:"-"`
	Namespace      string `json:"namespace,omitempty"`
	ObjectSelector string `json:"selector,omitempty"`
}

// NewGetDaemonStatus allocates a DaemonStatusOptions struct and sets
// default values to its keys.
func (a API) NewGetDaemonStatus() *GetDaemonStatus {
	return &GetDaemonStatus{
		API:            a,
		Namespace:      "",
		ObjectSelector: "*",
	}
}

// Do fetchs the daemon status structure from the agent api
func (o GetDaemonStatus) Do() (cluster.Status, error) {
	var ds cluster.Status
	opts := o.API.NewRequest()
	opts.Action = "daemon_status"
	opts.Options["namespace"] = o.Namespace
	opts.Options["selector"] = o.ObjectSelector
	b, err := o.API.Requester.Get(*opts)
	if err != nil {
		return ds, err
	}
	err = json.Unmarshal(b, &ds)
	if err != nil {
		return ds, err
	}
	return ds, nil
}
