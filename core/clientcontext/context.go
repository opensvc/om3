package clientcontext

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/opensvc/om3/core/env"
	"sigs.k8s.io/yaml"

	"github.com/mitchellh/go-homedir"
)

type (
	// config is the structure stored in and loaded from "~/.opensvc/config".
	// It contains the credentials and endpoint information to connect to
	// remote clusters.
	config struct {
		Contexts map[string]relation `json:"contexts"`
		Clusters map[string]cluster  `json:"clusters"`
		Users    map[string]user     `json:"users"`
	}

	// T is a dereferenced Cluster-User relation.
	T struct {
		Cluster   cluster `json:"cluster"`
		User      user    `json:"user"`
		Namespace string  `json:"namespace"`
	}

	// relation is a Cluster-User relation.
	relation struct {
		ClusterRefName string `json:"cluster"`
		UserRefName    string `json:"user"`
		Namespace      string `json:"namespace"`
	}

	// cluster host the endpoint address or name, and the certificate authority
	// to trust.
	cluster struct {
		CertificateAuthority string `json:"certificate_authority,omitempty"`
		Server               string `json:"server"`
		InsecureSkipVerify   bool   `json:"insecure"`
	}

	// user hosts the certificate and private to use to connect to the remote
	// cluster.
	user struct {
		ClientCertificate string `json:"client_certificate"`
		ClientKey         string `json:"client_key"`
		Password          string `json:"password"`
		Name              string `json:"name"`
	}
)

var (
	// Err is raised when a context definition has issues.
	Err = errors.New("context error")

	// ConfigFilename is the file where the context information is stored
	ConfigFilename = "~/.opensvc/config"
)

// IsSet returns true if the OSVC_CONTEXT environment variable is set
func IsSet() bool {
	return env.Context() != ""
}

func Load() (config, error) {
	var cfg config
	cf, _ := homedir.Expand(ConfigFilename)
	b, err := os.ReadFile(cf)
	if err != nil {
		return cfg, err
	}
	decodeJSON := func() error {
		if err := json.Unmarshal(b, &cfg); err != nil {
			return fmt.Errorf("json: %w", err)
		}
		return nil
	}
	decodeYAML := func() error {
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return fmt.Errorf("yaml: %w", err)
		}
		return nil
	}
	decode := func() error {
		var errs error
		if err := decodeJSON(); err == nil {
			return nil
		} else {
			errs = errors.Join(errs, err)
		}
		if err := decodeYAML(); err == nil {
			return nil
		} else {
			errs = errors.Join(errs, err)
		}
		return fmt.Errorf("could not decode %s: %w", ConfigFilename, errs)
	}
	if err := decode(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// New return a remote cluster connection context (endpoint and user)
func New() (T, error) {
	var c T
	n := env.Context()
	if n == "" {
		return c, nil
	}
	cfg, err := Load()
	if err != nil {
		return c, err
	}
	cr, ok := cfg.Contexts[n]
	if !ok {
		return c, fmt.Errorf("%w: context not defined: %s", Err, n)
	}
	c.Cluster, ok = cfg.Clusters[cr.ClusterRefName]
	if !ok {
		return c, fmt.Errorf("%w: cluster not defined: %s", Err, cr.ClusterRefName)
	}
	if c.Cluster.Server == "" {
		// If the cluster server is not specified, use the map key
		c.Cluster.Server = cr.ClusterRefName
	}
	if cr.UserRefName != "" {
		c.User, ok = cfg.Users[cr.UserRefName]
		if !ok {
			return c, fmt.Errorf("%w: user not defined: %s", Err, cr.ClusterRefName)
		}
	}
	c.Namespace = cr.Namespace
	if c.User.Name == "" {
		// If the user name is not specified, use the map key
		c.User.Name = cr.UserRefName
	}
	return c, nil
}

func (t T) String() string {
	b, _ := json.Marshal(t)
	return string(b)
}
