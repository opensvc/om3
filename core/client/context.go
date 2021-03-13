package client

import (
	"encoding/json"
	"os"

	"github.com/rs/zerolog/log"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
)

const (
	contextNameEnvVar = "OSVC_CONTEXT"
)

type (
	// ContextsConfig is the structure stored in and loaded from "~/.opensvc/config".
	// It contains the credentials and endpoint information to connect to
	// remote clusters.
	ContextsConfig struct {
		Contexts map[string]ContextRelation `json:"contexts"`
		Clusters map[string]Cluster         `json:"clusters"`
		Users    map[string]User            `json:"users"`
	}

	// Context is a dereferenced Cluster-User relation.
	Context struct {
		Cluster   Cluster `json:"cluster"`
		User      User    `json:"user"`
		Namespace string  `json:"namespace"`
	}

	// ContextRelation is a Cluster-User relation.
	ContextRelation struct {
		ClusterRefName string `json:"cluster"`
		UserRefName    string `json:"user"`
		Namespace      string `json:"namespace"`
	}

	// Cluster host the endpoint address or name, and the certificate authority
	// to trust.
	Cluster struct {
		CertificateAuthority string `json:"certificate_authority,omitempty"`
		Server               string `json:"server"`
		InsecureSkipVerify   bool   `json:"insecure"`
	}

	// User hosts the certificate and private to use to connect to the remote
	// cluster.
	User struct {
		ClientCertificate string `json:"client_certificate"`
		ClientKey         string `json:"client_key"`
	}
)

var (
	// ErrContext is raised when a context definition has issues.
	ErrContext = errors.New("context error")
)

// WantContext returns true if the OSVC_CONTEXT environment variable is set
func WantContext() bool {
	return os.Getenv(contextNameEnvVar) != ""
}

// NewContext return a remote cluster connection context (endpoint and user)
func NewContext() (Context, error) {
	var cfg ContextsConfig
	var c Context
	n := os.Getenv(contextNameEnvVar)
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
		return c, errors.Wrapf(ErrContext, "context not defined: %s", n)
	}
	c.Cluster, ok = cfg.Clusters[cr.ClusterRefName]
	if !ok {
		return c, errors.Wrapf(ErrContext, "cluster not defined: %s", cr.ClusterRefName)
	}
	if cr.UserRefName != "" {
		c.User, ok = cfg.Users[cr.UserRefName]
		if !ok {
			return c, errors.Wrapf(ErrContext, "user not defined: %s", cr.ClusterRefName)
		}
	}
	c.Namespace = cr.Namespace
	log.Debug().Msgf("new context: %s", c)
	return c, nil
}

func (t Context) String() string {
	b, _ := json.Marshal(t)
	return string(b)
}
