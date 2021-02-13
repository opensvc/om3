package client

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"

	"golang.org/x/net/http2"
)

type (
	// Type is the agent api client struct
	Type struct {
		Client *http.Client
		URL    string
	}
	Config struct {
		URL                string
		InsecureSkipVerify bool
	}
)

// New allocates a new agent api client struct
func New(c Config) Type {

	client := &http.Client{}
	client.Transport = &http2.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: c.InsecureSkipVerify,
		},
	}
	if c.URL == "" {
		c.URL = "https://127.0.0.1:1215"
	}
	t := Type{
		Client: client,
		URL:    c.URL,
	}
	return t
}

// Close closes the http.Client embedded in this agent client
func (t Type) Close() {

}

// DaemonStatus fetchs the daemon status structure from the agent api
func (t Type) DaemonStatus() (interface{}, error) {
	resp, err := t.Client.Get(t.URL + "/daemon_status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	fmt.Printf(
		"Got response %d: %s %s\n",
		resp.StatusCode, resp.Proto, string(body))
	return nil, nil
}
