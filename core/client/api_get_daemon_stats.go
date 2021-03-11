package client

// GetDaemonStats describes the daemon statistics api handler options.
type GetDaemonStats struct {
	API            API    `json:"-"`
	NodeSelector   string `json:"node"`
	ObjectSelector string `json:"selector"`
	Server         string `json:"server"`
}

// NewGetDaemonStats allocates a DaemonStatsCmdConfig struct and sets
// default values to its keys.
func (a API) NewGetDaemonStats() *GetDaemonStats {
	return &GetDaemonStats{
		API:            a,
		NodeSelector:   "*",
		ObjectSelector: "**",
		Server:         "",
	}
}

// Do fetchs the daemon statistics structure from the agent api
func (o GetDaemonStats) Do() ([]byte, error) {
	opts := NewRequest()
	opts.Node = "*"
	opts.Action = "daemon_stats"
	return o.API.Get(*opts)
}
