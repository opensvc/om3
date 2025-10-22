package cluster

import (
	"github.com/opensvc/om3/util/file"
)

type (
	Nodes []string

	// Config describes the cluster id, name and nodes
	// The cluster name is used as the right most part of cluster dns
	// names.
	Config struct {
		Issues     []string       `json:"issues"`
		ID         string         `json:"id"`
		Name       string         `json:"name"`
		Nodes      Nodes          `json:"nodes"`
		DNS        []string       `json:"dns"`
		CASecPaths []string       `json:"ca_sec_paths"`
		Listener   ConfigListener `json:"listener"`
		Quorum     bool           `json:"quorum"`

		// fields private, no exposed in daemon data
		// json nor events
		secret string

		sshKeyFile string
	}

	// ConfigHeartbeat represents the configuration for managing cluster heartbeat.
	ConfigHeartbeat struct {
		// CurrentSecretVersion represents the current version of the heartbeat secret used by
		// localhost to encrypt the heartbeat messages.
		CurrentSecretVersion uint64 `json:"current_secret_version"`

		// SecretSig represents the signature associated with the current configuration
		// of cluster heartbeat secrets.
		SecretSig string `json:"secret_sig"`

		// NextSecretVersion represents the version of the next heartbeat secret used by
		// localhost to encrypt the heartbeat messages after heartbeat secret rotation.
		NextSecretVersion uint64 `json:"next_secret_version,omitempty"`

		// These fields are private and not exposed in the daemon’s data, JSON output, or events
		currentSecret string
		nextSecret    string
	}

	ConfigListener struct {
		CRL            string `json:"crl"`
		Addr           string `json:"addr"`
		Port           int    `json:"port"`
		OpenIDIssuer   string `json:"openid_issuer"`
		OpenIDClientID string `json:"openid_client_id"`
		DNSSockGID     string `json:"dns_sock_gid"`
		DNSSockUID     string `json:"dns_sock_uid"`
	}
)

func (t Config) Secret() string {
	return t.secret
}

func (t *Config) SetSecret(s string) {
	t.secret = s
}

func (t *Config) SetSSHKeyFile(s string) {
	t.sshKeyFile = s
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
		secret:     t.secret,
		sshKeyFile: t.sshKeyFile,
	}
}

// SSHKeyFile returns the configured SSH key file path and a boolean indicating
// if the file exists and is regular.
func (t *Config) SSHKeyFile() (string, bool) {
	ok, _ := file.ExistsAndRegular(t.sshKeyFile)
	return t.sshKeyFile, ok
}
