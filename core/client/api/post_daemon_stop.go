package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// PostDaemonStop describes the daemon stop api handler options.
type PostDaemonStop struct {
	Base
	NodeSelector   string `json:"node"`
	ObjectSelector string `json:"selector"`
	Server         string `json:"server"`
}

// NewPostDaemonStop allocates a PostDaemonStop struct and sets
// default values to its keys.
func NewPostDaemonStop(t Getter) *PostDaemonStop {
	r := &PostDaemonStop{
		NodeSelector:   "",
		ObjectSelector: "",
		Server:         "",
	}
	r.SetClient(t)
	r.SetAction("/daemon/stop")
	r.SetMethod("POST")
	return r
}

// Do stop daemon
func (t PostDaemonStop) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
