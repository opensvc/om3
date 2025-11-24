package clientcontext

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/util/duration"

	"github.com/mitchellh/go-homedir"
)

type (
	// config is the structure stored in and loaded from "~/.opensvc/config".
	// It contains the credentials and endpoint information to connect to
	// remote clusters.
	config struct {
		Contexts map[string]Relation `json:"contexts"`
		Clusters map[string]Cluster  `json:"clusters"`
		Users    map[string]User     `json:"users"`
	}

	// T is a dereferenced Cluster-User relation.
	T struct {
		Cluster   Cluster `json:"cluster"`
		User      User    `json:"user"`
		Namespace string  `json:"namespace"`
	}

	// Relation is a Cluster-User relation.
	Relation struct {
		ClusterRefName       string             `json:"cluster"`
		UserRefName          string             `json:"user"`
		Namespace            *string            `json:"namespace,omitempty"`
		AccessTokenDuration  *duration.Duration `json:"access_token_duration,omitempty"`
		RefreshTokenDuration *duration.Duration `json:"refresh_token_duration,omitempty"`
	}

	// Cluster host the endpoint address or name, and the certificate authority
	// to trust.
	Cluster struct {
		CertificateAuthority *string `json:"certificate_authority,omitempty"`
		Server               string  `json:"server"`
		InsecureSkipVerify   *bool   `json:"insecure,omitempty"`
	}

	// User hosts the certificate and private to use to connect to the remote
	// cluster.
	User struct {
		ClientCertificate *string `json:"client_certificate,omitempty"`
		ClientKey         *string `json:"client_key,omitempty"`
		Name              *string `json:"name,omitempty"`
	}

	TokenInfo struct {
		Name            string `json:"name"`
		AccessExpireAt  string `json:"access_expired_at"`
		RefreshExpireAt string `json:"refresh_expired_at"`
		Authenticated   bool   `json:"authenticated"`
		AuthenticatedAt string `json:"authenticated_at"`
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
		cfg.Clusters = make(map[string]Cluster)
	}
	if cfg.Users == nil {
		cfg.Users = make(map[string]User)
	}
	if cfg.Contexts == nil {
		cfg.Contexts = make(map[string]Relation)
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
	c.Namespace = *cr.Namespace
	if c.User.Name == nil || *c.User.Name == "" {
		// If the user name is not specified, use the map key
		c.User.Name = &cr.UserRefName
	}
	return c, nil
}

func (t T) String() string {
	b, _ := json.Marshal(t)
	return string(b)
}

func (t *TokenInfo) Unstructured() map[string]any {
	return map[string]any{
		"name":               t.Name,
		"access_expired_at":  t.AccessExpireAt,
		"refresh_expired_at": t.RefreshExpireAt,
		"authenticated":      t.Authenticated,
		"authenticated_at":   t.AuthenticatedAt,
	}
}

func (c config) Save() error {
	tempFilename, _ := homedir.Expand(ConfigFilename + ".tmp")
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tempFilename, b, 0o600); err != nil {
		return err
	}

	filename, _ := homedir.Expand(ConfigFilename + ".json")
	if err := os.Rename(tempFilename, filename); err != nil {
		return err
	}

	return nil
}

func setItem[V any](m map[string]V, name string, item V, mustExist bool, singular, plural string) error {
	if m == nil {
		return fmt.Errorf("%w: no %s defined", Err, plural)
	}
	_, exists := m[name]
	if mustExist && !exists {
		return fmt.Errorf("%w: %s %s does not exist", Err, singular, name)
	}
	if !mustExist && exists {
		return fmt.Errorf("%w: %s %s already exists", Err, singular, name)
	}
	m[name] = item
	return nil
}

func removeItem[V any](m map[string]V, name string, singular, plural string) error {
	if m == nil {
		return fmt.Errorf("%w: no %s defined", Err, plural)
	}
	_, exists := m[name]
	if !exists {
		return fmt.Errorf("%w: %s %s does not exist", Err, singular, name)
	}
	delete(m, name)
	return nil
}

func (c config) AddContext(name string, r Relation) error {
	return setItem(c.Contexts, name, r, false, "context", "contexts")
}

func (c config) ChangeContext(name string, r Relation) error {
	return setItem(c.Contexts, name, r, true, "context", "contexts")
}

func (c config) RemoveContext(name string) error {
	return removeItem(c.Contexts, name, "context", "contexts")
}

func (c config) AddCluster(name string, cl Cluster) error {
	return setItem(c.Clusters, name, cl, false, "cluster", "clusters")
}

func (c config) ChangeCluster(name string, cl Cluster) error {
	return setItem(c.Clusters, name, cl, true, "cluster", "clusters")
}

func (c config) RemoveCluster(name string) error {
	return removeItem(c.Clusters, name, "cluster", "clusters")
}

func (c config) ClusterUsed(name string) (bool, error) {
	if c.Contexts == nil {
		return false, nil
	}
	for _, r := range c.Contexts {
		if r.ClusterRefName == name {
			return true, nil
		}
	}
	return false, nil
}

func (c config) AddUser(name string, u User) error {
	return setItem(c.Users, name, u, false, "user", "users")
}

func (c config) ChangeUser(name string, u User) error {
	return setItem(c.Users, name, u, true, "user", "users")
}

func (c config) RemoveUser(name string) error {
	return removeItem(c.Users, name, "user", "users")
}

func (c config) UserUsed(name string) (bool, error) {
	if c.Contexts == nil {
		return false, nil
	}
	for _, r := range c.Contexts {
		if r.UserRefName == name {
			return true, nil
		}
	}
	return false, nil
}
