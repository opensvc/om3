package api

import (
	"github.com/opensvc/om3/core/client/request"
)

// GetDaemonStats describes the daemon statistics api handler options.
type GetDaemonStats struct {
	Base
	NodeSelector   string `json:"node"`
	ObjectSelector string `json:"selector"`
	Server         string `json:"server"`
}

// NewGetDaemonStats allocates a DaemonStatsCmdConfig struct and sets
// default values to its keys.
func NewGetDaemonStats(t Getter) *GetDaemonStats {
	r := &GetDaemonStats{
		NodeSelector:   "*",
		ObjectSelector: "**",
		Server:         "",
	}
	r.SetClient(t)
	r.SetAction("daemon_stats")
	r.SetMethod("GET")
	r.SetNode("*")
	return r
}

// Do fetchs the daemon statistics structure from the agent api
func (t GetDaemonStats) Do() ([]byte, error) {
	req := request.NewFor(t)
	return Route(t.client, *req)
}
