package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"opensvc.com/opensvc/core/types"
)

// DaemonStatsConfig
type DaemonStatsCmdConfig struct {
	NodeSelector   string
	ObjectSelector string
	Server         string
}

// NewDaemonStatsCmdConfig allocates a DaemonStatsCmdConfig struct and sets
// default values to its keys.
func NewDaemonStatsCmdConfig() *DaemonStatsCmdConfig {
	return &DaemonStatsCmdConfig{
		NodeSelector:   "*",
		ObjectSelector: "**",
		Server:         "",
	}
}

// DaemonStats fetchs the daemon statistics structure from the agent api
func (a API) DaemonStats(c DaemonStatsCmdConfig) (types.DaemonStats, error) {
	type t struct {
		Status int               `json:"status"`
		Data   types.DaemonStats `json:"data"`
	}
	var ds t
	resp, err := a.Requester.Get("daemon_stats")
	if err != nil {
		fmt.Println(err)
		return ds.Data, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return ds.Data, err
	}
	body = bytes.TrimRight(body, "\x00")
	err = json.Unmarshal(body, &ds)
	if err != nil {
		fmt.Println(err)
		return ds.Data, err
	}
	fmt.Println(ds)
	return ds.Data, nil
}
