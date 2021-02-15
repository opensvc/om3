package client

import (
	"fmt"
	"io/ioutil"
)

// DaemonStatus fetchs the daemon status structure from the agent api
func (a API) DaemonStatus() (interface{}, error) {
	resp, err := a.Requester.Get("daemon_status")
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	fmt.Printf(
		"Got response %d: %s %s\n",
		resp.StatusCode, resp.Proto, string(body))
	return nil, nil
}
