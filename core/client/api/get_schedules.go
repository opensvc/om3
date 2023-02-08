package api

import (
	"github.com/opensvc/om3/core/client/request"
)

// GetDaemonStats describes the daemon statistics api handler options.
type GetSchedules struct {
	Base
	NodeSelector   string `json:"node"`
	ObjectSelector string `json:"selector"`
	Namespace      string `json:"namespace"`
	Server         string `json:"server"`
}

// NewGetDaemonStats allocates a DaemonStatsCmdConfig struct and sets
// default values to its keys.
func NewGetSchedules(t Getter) *GetSchedules {
	r := &GetSchedules{
		NodeSelector:   "",
		ObjectSelector: "",
		Server:         "",
	}
	r.SetClient(t)
	r.SetAction("schedules")
	r.SetMethod("GET")
	//r.SetNode("*")
	return r
}

// Do fetchs the daemon statistics structure from the agent api
func (t GetSchedules) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
