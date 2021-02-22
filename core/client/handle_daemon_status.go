package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"opensvc.com/opensvc/core/cluster"
)

// DaemonStatusOptions describes the daemon status api handler options.
type DaemonStatusOptions struct {
	Namespace      string `json:"namespace,omitempty"`
	ObjectSelector string `json:"selector,omitempty"`
}

// NewDaemonStatusOptions allocates a DaemonStatusOptions struct and sets
// default values to its keys.
func NewDaemonStatusOptions() *DaemonStatusOptions {
	return &DaemonStatusOptions{
		Namespace:      "",
		ObjectSelector: "*",
	}
}

// DaemonStatus fetchs the daemon status structure from the agent api
func (a API) DaemonStatus(o DaemonStatusOptions) (cluster.Status, error) {
	var ds cluster.Status
	opts := a.NewRequest()
	opts.Action = "daemon_status"
	opts.Options["namespace"] = o.Namespace
	opts.Options["selector"] = o.ObjectSelector
	resp, err := a.Requester.Get(*opts)
	if err != nil {
		fmt.Println(err)
		return ds, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return ds, err
	}
	body = bytes.TrimRight(body, "\x00")
	err = json.Unmarshal(body, &ds)
	if err != nil {
		fmt.Println(err)
		return ds, err
	}
	return ds, nil
}
