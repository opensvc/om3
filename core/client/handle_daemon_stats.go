package client

import (
	"encoding/json"

	"opensvc.com/opensvc/core/cluster"
)

// GetDaemonStats describes the daemon statistics api handler options.
type GetDaemonStats struct {
	API            API    `json:"-"`
	NodeSelector   string `json:"node"`
	ObjectSelector string `json:"selector"`
	Server         string `json:"server"`
}

// NewGetDaemonStats allocates a DaemonStatsCmdConfig struct and sets
// default values to its keys.
func (a API) NewGetDaemonStats() *GetDaemonStats {
	return &GetDaemonStats{
		API:            a,
		NodeSelector:   "*",
		ObjectSelector: "**",
		Server:         "",
	}
}

// Do fetchs the daemon statistics structure from the agent api
func (o GetDaemonStats) Do() (cluster.Stats, error) {
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
	opts := o.API.NewRequest()
	opts.Node = "*"
	opts.Action = "daemon_stats"

	b, err := o.API.Requester.Get(*opts)
	if err != nil {
		return ds, err
	}
	err = json.Unmarshal(b, &t)
	if err != nil {
		return ds, err
	}
	for k, v := range t.Nodes {
		ds[k] = v.Data
	}
	return ds, nil
}
