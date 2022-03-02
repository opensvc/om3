package api

import (
	"opensvc.com/opensvc/core/api/apimodel"
	"opensvc.com/opensvc/core/client/request"
)

type (
	// GetDaemonRunning describes the daemon running api handler options.
	GetDaemonRunning struct {
		Base
		NodeSelector   string `json:"node"`
		ObjectSelector string `json:"selector"`
		Server         string `json:"server"`
	}

	GetDaemonRunningData []struct {
		apimodel.BaseResponseMuxData
		Data bool `json:"data,omitempty"`
	}
	// GetDaemonRunningResponse
	GetDaemonRunningResponse struct {
		apimodel.BaseResponseMux
		Data GetDaemonRunningData `json:"data"`
	}
)

// NewGetDaemonRunning allocates a GetDaemonRunning struct and sets
// default values to its keys.
func NewGetDaemonRunning(t Getter) *GetDaemonRunning {
	r := &GetDaemonRunning{
		NodeSelector:   "",
		ObjectSelector: "",
		Server:         "",
	}
	r.SetClient(t)
	r.SetAction("daemon/running")
	r.SetMethod("GET")
	return r
}

// Do get daemon running
func (t GetDaemonRunning) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
