package api

import (
	"opensvc.com/opensvc/core/client/request"
)

// GetDaemonRunning describes the daemon running api handler options.
type GetDaemonRunning struct {
	Base
	NodeSelector   string `json:"node"`
	ObjectSelector string `json:"selector"`
	Server         string `json:"server"`
}

// NewGetDaemonRunning allocates a GetDaemonRunning struct and sets
// default values to its keys.
func NewGetDaemonRunning(t Getter) *GetDaemonRunning {
	r := &GetDaemonRunning{
		NodeSelector:   "*",
		ObjectSelector: "**",
		Server:         "",
	}
	r.SetClient(t)
	r.SetAction("daemon_running")
	r.SetMethod("GET")
	r.SetNode("*")
	return r
}

// Do get daemon running
func (t GetDaemonRunning) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
