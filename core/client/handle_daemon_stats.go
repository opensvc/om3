package client

import (
	"bytes"
	"encoding/json"
	"io/ioutil"

	"opensvc.com/opensvc/core/cluster"
)

// DaemonStatsOptions describes the daemon statistics api handler options.
type DaemonStatsOptions struct {
	NodeSelector   string
	ObjectSelector string
	Server         string
}

// NewDaemonStatsOptions allocates a DaemonStatsCmdConfig struct and sets
// default values to its keys.
func NewDaemonStatsOptions() *DaemonStatsOptions {
	return &DaemonStatsOptions{
		NodeSelector:   "*",
		ObjectSelector: "**",
		Server:         "",
	}
}

// DaemonStats fetchs the daemon statistics structure from the agent api
func (a API) DaemonStats(o DaemonStatsOptions) (cluster.Stats, error) {
	type nodeData struct {
		Status int               `json:"status"`
		Data   cluster.NodeStats `json:"data"`
	}
	type responseType struct {
		Status int                 `json:"status"`
		Nodes  map[string]nodeData `json:"nodes"`
	}
	ds := make(cluster.Stats)
	var t responseType
	opts := a.NewRequest()
	opts.Node = "*"
	opts.Action = "daemon_stats"

	resp, err := a.Requester.Get(*opts)
	if err != nil {
		return ds, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ds, err
	}
	body = bytes.TrimRight(body, "\x00")
	err = json.Unmarshal(body, &t)
	if err != nil {
		return ds, err
	}
	for k, v := range t.Nodes {
		ds[k] = v.Data
	}
	return ds, nil
}
