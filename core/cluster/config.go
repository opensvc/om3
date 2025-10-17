package cluster

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
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
		Issues     []string        `json:"issues"`
		ID         string          `json:"id"`
		Name       string          `json:"name"`
		Nodes      Nodes           `json:"nodes"`
		DNS        []string        `json:"dns"`
		CASecPaths []string        `json:"ca_sec_paths"`
		Heartbeat  ConfigHeartbeat `json:"hb"`
		Listener   ConfigListener  `json:"listener"`
		Quorum     bool            `json:"quorum"`

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

func (t *ConfigHeartbeat) Secrets() (currentVersion uint64, currentSecret string, NextVersion uint64, nextSecret string) {
	if t == nil {
		return
	}
	return t.CurrentSecretVersion, t.currentSecret, t.NextSecretVersion, t.nextSecret
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
		Heartbeat:  t.Heartbeat,
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

func (t *Config) SetHeartbeatSecret(s string) error {
	// reset values
	hbCfg := t.Heartbeat
	hbCfg.currentSecret = ""
	hbCfg.nextSecret = ""
	hbCfg.CurrentSecretVersion = 0
	hbCfg.NextSecretVersion = 0
	hbCfg.SecretSig = ""

	l := strings.Fields(s)
	if len(l) >= 1 {
		i, v, err := unpackSecret(l[0])
		if err != nil {
			return fmt.Errorf("failed to unpack rolling secret Value: %w", err)
		} else {
			hbCfg.currentSecret = v
			hbCfg.CurrentSecretVersion = i
		}
	}
	if len(l) > 1 {
		i, v, err := unpackSecret(l[1])
		if err != nil {
			return fmt.Errorf("failed to unpack next secret Value: %w", err)
		} else {
			hbCfg.nextSecret = v
			hbCfg.NextSecretVersion = i
		}
	}

	if len(hbCfg.currentSecret) == 0 {
		return errors.New("current secret is empty")
	}

	if len(hbCfg.currentSecret) > 0 || len(hbCfg.nextSecret) > 0 || hbCfg.CurrentSecretVersion > 0 || hbCfg.NextSecretVersion > 0 {
		sigToSum := fmt.Sprintf("%d:%s %d:%s", hbCfg.CurrentSecretVersion, hbCfg.currentSecret, hbCfg.NextSecretVersion, hbCfg.nextSecret)
		sha256sum := sha256.Sum256([]byte(sigToSum))
		hbCfg.SecretSig = base64.RawStdEncoding.EncodeToString(sha256sum[:])
	}

	t.Heartbeat = hbCfg
	return nil
}

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
