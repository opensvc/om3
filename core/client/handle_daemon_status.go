package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"opensvc.com/opensvc/core/cluster"
)

// DaemonStatus fetchs the daemon status structure from the agent api
func (a API) DaemonStatus() (cluster.Status, error) {
	var ds cluster.Status
	opts := a.NewRequestOptions()
	resp, err := a.Requester.Get("daemon_status", *opts)
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
