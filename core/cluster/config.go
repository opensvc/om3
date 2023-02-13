package cluster

type (
	Nodes []string

	// Config describes the cluster id, name and nodes
	// The cluster name is used as the right most part of cluster dns
	// names.
	Config struct {
		ID         string         `json:"id"`
		Name       string         `json:"name"`
		Nodes      Nodes          `json:"nodes"`
		DNS        []string       `json:"dns"`
		CASecPaths []string       `json:"ca_sec_paths"`
		Listener   ConfigListener `json:"listener"`

		// fields private, no exposed in daemon data
		// json nor events
		secret string
	}
	ConfigListener struct {
		CRL             string `json:"crl"`
		Addr            string `json:"addr"`
		Port            int    `json:"port"`
		TLSAddr         string `json:"tls_addr"`
		TLSPort         int    `json:"tls_port"`
		OpenIdWellKnown string `json:"openid_well_known"`
	}
)

func (t Config) Secret() string {
	return t.secret
}

func (t *Config) SetSecret(s string) {
	t.secret = s
}

func (t Nodes) Contains(s string) bool {
	for _, nodename := range t {
		if nodename == s {
			return true
		}
	}
	return false
}
