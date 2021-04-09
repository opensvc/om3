package client

// GetDaemonStats describes the daemon statistics api handler options.
type GetDaemonStats struct {
	client         *T
	NodeSelector   string `json:"node"`
	ObjectSelector string `json:"selector"`
	Server         string `json:"server"`
}

// NewGetDaemonStats allocates a DaemonStatsCmdConfig struct and sets
// default values to its keys.
func (t *T) NewGetDaemonStats() *GetDaemonStats {
	return &GetDaemonStats{
		client:         t,
		NodeSelector:   "*",
		ObjectSelector: "**",
		Server:         "",
	}
}

// Do fetchs the daemon statistics structure from the agent api
func (o GetDaemonStats) Do() ([]byte, error) {
	req := NewRequestFor("daemon_stats", o)
	req.Node = "*"
	return o.client.Get(*req)
}
