package cluster

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

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

		hbSecret HeartbeatSecret

		sshKeyFile string
	}

	HeartbeatSecret struct {
		Value     string
		Gen       uint64
		NextValue string
		NextGen   uint64
		Sig       string
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

func unpackSecret(s string) (uint64, string, error) {
	parts := strings.SplitN(s, ":", 2)

	if len(parts) == 1 {
		return 0, parts[0], nil
	} else if uintPart, err := strconv.ParseUint(parts[0], 10, 64); err != nil {
		return 0, "", errors.New("failed to convert first part to uint64")
	} else {
		return uintPart, parts[1], nil
	}
}

func UnpackHeartbeatSecret(s string) (HeartbeatSecret, error) {
	secret := HeartbeatSecret{}
	l := strings.Fields(s)
	if len(l) >= 1 {
		i, v, err := unpackSecret(l[0])
		if err != nil {
			return secret, fmt.Errorf("failed to unpack rolling secret Value: %w", err)
		} else {
			secret.Value = v
			secret.Gen = i
		}
	}
	if len(l) > 1 {
		i, v, err := unpackSecret(l[1])
		if err != nil {
			return secret, fmt.Errorf("failed to unpack next secret Value: %w", err)
		} else {
			secret.NextValue = v
			secret.NextGen = i
		}
	}

	if len(secret.Value) > 0 || len(secret.NextValue) > 0 {
		sha256sum := sha256.Sum256([]byte(secret.Value + ":" + secret.NextValue))
		secret.Sig = base64.RawStdEncoding.EncodeToString(sha256sum[:])
	}

	return secret, nil
}

func (t Config) Secret() string {
	return t.secret
}

func (t *Config) SetSecret(s string) {
	t.secret = s
}

func (t *Config) HeartbeatSecret() HeartbeatSecret {
	if t == nil {
		return HeartbeatSecret{}
	}
	return t.hbSecret
}

func (t *Config) SetHeartbeatSecret(s HeartbeatSecret) {
	fmt.Fprintf(os.Stderr, "SetHeartbeatSecret %#v\n", s)
	t.hbSecret = s
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
		hbSecret:   t.hbSecret,
		sshKeyFile: t.sshKeyFile,
	}
}

// SSHKeyFile returns the configured SSH key file path and a boolean indicating
// if the file exists and is regular.
func (t *Config) SSHKeyFile() (string, bool) {
	ok, _ := file.ExistsAndRegular(t.sshKeyFile)
	return t.sshKeyFile, ok
}
