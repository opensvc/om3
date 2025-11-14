package clientcontext

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/opensvc/om3/core/env"

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
		Name              string `json:"name"`
	}
)

var (
	// Err is raised when a context definition has issues.
	Err = errors.New("context error")

	ConfigFolder = "~/.config/opensvc/"
	// ConfigFilename is the file where the context information is stored
	ConfigFilename = ConfigFolder + "contexts"
)

// IsSet returns true if the OSVC_CONTEXT environment variable is set
func IsSet() bool {
	return env.Context() != ""
}

func Load() (config, error) {
	var errs error
	var cfg config
	filenames := []string{
		ConfigFilename,
		ConfigFilename + ".json",
		ConfigFilename + ".yaml",
	}
	for _, filename := range filenames {
		filename, _ := homedir.Expand(filename)
		if err := loadFile(filename, &cfg); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return cfg, errs
}

func loadFile(name string, cfg *config) error {
	var (
		tryJSON, tryYAML bool
		this             config
	)
	b, err := os.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	decodeJSON := func() error {
		if err := json.Unmarshal(b, &this); err != nil {
			return fmt.Errorf("json: %w", err)
		}
		return nil
	}
	decodeYAML := func() error {
		if err := yaml.Unmarshal(b, &this); err != nil {
			return fmt.Errorf("yaml: %w", err)
		}
		return nil
	}
	decode := func() error {
		var errs error
		if strings.HasSuffix(name, ".json") {
			tryJSON = true
		} else if strings.HasSuffix(name, ".yaml") {
			tryYAML = true
		} else {
			tryJSON = true
			tryYAML = true
		}

		if tryJSON {
			if err := decodeJSON(); err == nil {
				return nil
			} else {
				errs = errors.Join(errs, err)
			}
		}
		if tryYAML {
			if err := decodeYAML(); err == nil {
				return nil
			} else {
				errs = errors.Join(errs, err)
			}
		}
		return fmt.Errorf("could not decode %s: %w", name, errs)
	}
	if err := decode(); err != nil {
		return err
	}
	if cfg.Clusters == nil {
		cfg.Clusters = make(map[string]cluster)
	}
	if cfg.Users == nil {
		cfg.Users = make(map[string]user)
	}
	if cfg.Contexts == nil {
		cfg.Contexts = make(map[string]relation)
	}
	for k, v := range this.Clusters {
		cfg.Clusters[k] = v
	}
	for k, v := range this.Users {
		cfg.Users[k] = v
	}
	for k, v := range this.Contexts {
		cfg.Contexts[k] = v
	}
	return nil
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
