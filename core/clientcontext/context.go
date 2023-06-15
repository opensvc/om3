package clientcontext

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/opensvc/om3/core/env"
	"github.com/rs/zerolog/log"

	"github.com/mitchellh/go-homedir"
)

type (
	// config is the structure stored in and loaded from "~/.opensvc/config".
	// It contains the credentials and endpoint information to connect to
	// remote clusters.
	config struct {
		Contexts map[string]relation `json:"contexts" yaml:"contexts"`
		Clusters map[string]cluster  `json:"clusters" yaml:"clusters"`
		Users    map[string]user     `json:"users" yaml:"users"`
	}

	// T is a dereferenced Cluster-User relation.
	T struct {
		Cluster   cluster `json:"cluster" yaml:"cluster"`
		User      user    `json:"user" yaml:"user"`
		Namespace string  `json:"namespace" yaml:"namespace"`
	}

	// relation is a Cluster-User relation.
	relation struct {
		ClusterRefName string `json:"cluster" yaml:"cluster"`
		UserRefName    string `json:"user" yaml:"user"`
		Namespace      string `json:"namespace" yaml:"namespace"`
	}

	// cluster host the endpoint address or name, and the certificate authority
	// to trust.
	cluster struct {
		CertificateAuthority string `json:"certificate_authority,omitempty" yaml:"certificate_authority,omitempty"`
		Server               string `json:"server" yaml:"server"`
		InsecureSkipVerify   bool   `json:"insecure" yaml:"insecure"`
	}

	// user hosts the certificate and private to use to connect to the remote
	// cluster.
	user struct {
		ClientCertificate string `json:"client_certificate" yaml:"client_certificate"`
		ClientKey         string `json:"client_key" yaml:"client_key"`
	}
)

var (
	// Err is raised when a context definition has issues.
	Err = errors.New("context error")
)

// IsSet returns true if the OSVC_CONTEXT environment variable is set
func IsSet() bool {
	return env.Context() != ""
}

// New return a remote cluster connection context (endpoint and user)
func New() (T, error) {
	var cfg config
	var c T
	n := env.Context()
	if n == "" {
		return c, nil
	}
	cf, _ := homedir.Expand("~/.opensvc/config")
	f, err := os.Open(cf)
	if err != nil {
		return c, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	if err := dec.Decode(&cfg); err != nil {
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
	if cr.UserRefName != "" {
		c.User, ok = cfg.Users[cr.UserRefName]
		if !ok {
			return c, fmt.Errorf("%w: user not defined: %s", Err, cr.ClusterRefName)
		}
	}
	c.Namespace = cr.Namespace
	log.Debug().Msgf("New context: %s", c)
	return c, nil
}

func (t T) String() string {
	b, _ := json.Marshal(t)
	return string(b)
}
