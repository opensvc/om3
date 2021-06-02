package clientcontext

import (
	"encoding/json"
	"os"

	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/env"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
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
		return c, errors.Wrapf(Err, "context not defined: %s", n)
	}
	c.Cluster, ok = cfg.Clusters[cr.ClusterRefName]
	if !ok {
		return c, errors.Wrapf(Err, "cluster not defined: %s", cr.ClusterRefName)
	}
	if cr.UserRefName != "" {
		c.User, ok = cfg.Users[cr.UserRefName]
		if !ok {
			return c, errors.Wrapf(Err, "user not defined: %s", cr.ClusterRefName)
		}
	}
	c.Namespace = cr.Namespace
	log.Debug().Msgf("new context: %s", c)
	return c, nil
}

func (t T) String() string {
	b, _ := json.Marshal(t)
	return string(b)
}
