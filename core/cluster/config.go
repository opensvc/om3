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
		Quorum     bool           `json:"quorum"`
		Vip        Vip            `json:"vip"`

		// fields private, no exposed in daemon data
		// json nor events
		secret string
	}
	ConfigListener struct {
		CRL             string `json:"crl"`
		Addr            string `json:"addr"`
		Port            int    `json:"port"`
		OpenIDWellKnown string `json:"openid_well_known"`
		DNSSockGID      string `json:"dns_sock_gid"`
		DNSSockUID      string `json:"dns_sock_uid"`
	}

	// Vip struct describes cluster vip settings
	Vip struct {
		// Default is the default vip configuration value, must be not zero to
		// enable cluster vip
		Default string `json:"default"`
		// Addr is the default vip addr
		Addr string `json:"name"`
		// Netmask is the default vip netmask
		Netmask string `json:"netmask"`
		// Dev is the default vip device
		Dev string `json:"dev"`
		// Devs is a map of node names to custom vip device (when
		// the device for node name is not equal to default vip device)
		Devs map[string]string `json:"devs"`
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

func (t *Config) DeepCopy() *Config {
	return &Config{
		ID:         t.ID,
		Name:       t.Name,
		Nodes:      append(Nodes{}, t.Nodes...),
		DNS:        append([]string{}, t.DNS...),
		CASecPaths: append([]string{}, t.CASecPaths...),
		Listener:   t.Listener,
		Quorum:     t.Quorum,
		Vip:        *t.Vip.DeepCopy(),
		secret:     t.secret,
	}
}

func (v *Vip) DeepCopy() *Vip {
	newV := *v
	devs := make(map[string]string)
	for k, v := range v.Devs {
		devs[k] = v
	}
	newV.Devs = devs
	return &newV
}

func (v *Vip) Equal(o *Vip) bool {
	if (v == nil && o != nil) || (o == nil && v != nil) {
		return false
	}
	if v.Default != o.Default || v.Dev != o.Dev || v.Netmask != o.Netmask || v.Addr != o.Addr {
		return false
	}
	if len(v.Devs) != len(o.Devs) {
		return false
	}
	for k, value := range v.Devs {
		if oValue, ok := o.Devs[k]; !ok || oValue != value {
			return false
		}
	}
	return true
}
