package sgcp

import (
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// Config represents the SGCP configuration structure
	Config struct {
		Files        FilesConfig `yaml:"files"`
		DNS          DNSConfig   `yaml:"dns"`
		Auth         AuthConfig  `yaml:"auth"`
		Cache        CacheConfig `yaml:"cache"`
		DisabledFlag string      `yaml:"disabled_flag"`
	}

	// FilesConfig contains file-related API configuration
	FilesConfig struct {
		BaseURL string `yaml:"base_url"`
		Path    struct {
			FS     string `yaml:"fs"`
			Client string `yaml:"client"`
			CG     string `yaml:"cg"`
		} `yaml:"path"`
		FS struct {
			Permission     string   `yaml:"permission"`
			Protocol       string   `yaml:"protocol"`
			Exclusive      bool     `yaml:"exclusive"`
			IgnoredClients []string `yaml:"ignored_clients"`
		} `yaml:"fs"`
		Client struct{}
		CG     struct {
			Timeout string `yaml:"timeout"`
		} `yaml:"cg"`
	}

	// DNSConfig contains DNS-related API configuration
	DNSConfig struct {
		BaseURL    string `yaml:"base_url"`
		NoneTarget string `yaml:"none_target"`
		Path       struct {
			CName string `yaml:"cname"`
			Zone  string `yaml:"zone"`
		} `yaml:"path"`
	}

	// AuthConfig contains authentication configuration
	AuthConfig struct {
		BaseURL       string              `yaml:"base_url" json:"base_url"`
		DefaultSecret string              `yaml:"default_secret"`
		Scopes        map[string][]string `yaml:"scopes"`
		TTLSeconds    int                 `yaml:"ttl_seconds"`
		Timeout       int                 `yaml:"timeout"`
	}

	// CacheConfig contains caching configuration. A zero ttl_seconds
	// disables the caching.
	CacheConfig struct {
		TTLSeconds int `yaml:"ttl_seconds"`
	}
)

// Defaults for the files.fs settings. A driver keyword left unset falls back
// to the sgcp.yaml value, which itself falls back to these.
const (
	FsDefaultPermission = "read-write"
	FsDefaultProtocol   = "nfs4.1"

	// FsDefaultExclusive documents the files.fs.exclusive default. The
	// setting is a bool, so its zero value already is that default: no
	// code needs to apply it.
	FsDefaultExclusive = "false"

	// CGDefaultTimeout is the consistency group operations timeout used
	// when files.cg.timeout is unset or is not a parsable duration.
	CGDefaultTimeout = 30 * time.Minute
)

// FsIgnoredClients is the files.fs.ignored_clients default. It can not be a
// const, as a slice is not a constant expression.
var FsIgnoredClients = []string{}

// SGCPConfig is the global configuration instance
var (
	config     *Config
	configLck  sync.RWMutex
	configOnce sync.Once
	logger     = plog.NewDefaultLogger()

	DefaultConfigPath = "/etc/om3/sgcp.yaml"
)

// GetConfig returns the global SGCP configuration, loading it once
func GetConfig() *Config {
	configLck.Lock()
	defer configLck.Unlock()
	configOnce.Do(func() {
		var err error
		config, err = loadConfig(DefaultConfigPath)
		if err != nil {
			logger.Debugf("Failed to load SGCP config: %v", err)
		}
	})

	return config.Clone()
}

// SetConfigForTest sets the global configuration for testing,
// loading it from the specified file, or resetting it if null configFile is passed.
func SetConfigForTest(configFile string) {
	// Need to call GetConfig to ensure configOnce is initialized
	_ = GetConfig()

	configLck.Lock()
	defer configLck.Unlock()
	if configFile == "" {
		config = nil
		return
	}
	var err error
	config, err = loadConfig(configFile)
	if err != nil {
		logger.Debugf("Failed to load SGCP config: %v", err)
	}
}

func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	n := *c
	scopes := make(map[string][]string)
	for k, v := range c.Auth.Scopes {
		scopes[k] = append([]string{}, v...)
	}
	n.Auth.Scopes = scopes
	n.Files.FS.IgnoredClients = append([]string{}, c.Files.FS.IgnoredClients...)
	return &n
}

// setDefaults fills with their package default the settings the configuration
// file leaves unset, so a sgcp.yaml with no files.fs section still yields a
// usable configuration.
//
// files.fs.exclusive is absent here: it is a bool, and its zero value already
// is FsDefaultExclusive.
func (c *Config) setDefaults() {
	if c.Files.FS.Permission == "" {
		c.Files.FS.Permission = FsDefaultPermission
	}
	if c.Files.FS.Protocol == "" {
		c.Files.FS.Protocol = FsDefaultProtocol
	}
	if c.Files.FS.IgnoredClients == nil {
		c.Files.FS.IgnoredClients = append([]string{}, FsIgnoredClients...)
	}
}

// CGTimeout returns the consistency group operations timeout parsed from
// files.cg.timeout. A unset or unparsable value yields CGDefaultTimeout.
// The duration default unit is the second, so a bare "300" means 5 minutes.
func (c FilesConfig) CGTimeout() time.Duration {
	v, err := converters.Duration.Convert(c.CG.Timeout)
	if err != nil {
		return CGDefaultTimeout
	}
	d, ok := v.(*time.Duration)
	if !ok || d == nil {
		return CGDefaultTimeout
	}
	return *d
}

func (c *Config) WithAuthSecret(s string) *Config {
	c.Auth.DefaultSecret = s
	return c
}

func (c *Config) WithFileURL(s string) *Config {
	c.Files.BaseURL = s
	return c
}

func (c *Config) WithDNSBaseURL(s string) *Config {
	c.DNS.BaseURL = s
	return c
}

// GetScopes returns the auth scopes for a given scope type
func (c *Config) GetScopes(scopeType string) []string {
	if c == nil {
		return []string{}
	}
	if scopes, ok := c.Auth.Scopes[scopeType]; ok {
		return scopes
	}
	return []string{}
}

// GetDefaultSecret returns the default secret name
func (c *Config) GetDefaultSecret() string {
	if c == nil {
		return ""
	}
	return c.Auth.DefaultSecret
}

// loadConfig loads the SGCP configuration
func loadConfig(s string) (*Config, error) {
	data, err := os.ReadFile(s)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", s, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", s, err)
	}
	config.setDefaults()

	return &config, nil
}
