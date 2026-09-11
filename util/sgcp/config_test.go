package sgcp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/testsgcphelper"
)

func Setup(t *testing.T) func() {
	t.Helper()
	cfgFile := testsgcphelper.InstallConfig(t)
	SetConfigForTest(cfgFile)

	return func() {
		SetConfigForTest("")
	}
}

// TestGetScopes tests the GetScopes function
func TestGetScopes(t *testing.T) {
	defer Setup(t)()

	cfg := GetConfig()
	require.NotNil(t, cfg)

	scopes := cfg.GetScopes("custom_admin")
	assert.Equal(t, []string{"custom:read", "custom:write"}, scopes)

	// Test unknown scope
	scopes = cfg.GetScopes("unknown")
	assert.Equal(t, []string{}, scopes)
}

// TestGetDefaultSecret tests the GetDefaultSecret function
func TestGetDefaultSecret(t *testing.T) {
	defer Setup(t)()

	cfg := GetConfig()
	require.NotNil(t, cfg)

	secret := cfg.GetDefaultSecret()
	assert.Equal(t, "the-secret", secret)
}

// TestGetConfig tests the global config getter when no config exists
func TestGetConfigWhenNotPresent(t *testing.T) {
	SetConfigForTest("")
	cfg := GetConfig()
	assert.Nil(t, cfg)
}

// TestLoadConfig tests loading the configuration from a file
func TestLoadConfig(t *testing.T) {
	defer Setup(t)()

	cfg := GetConfig()
	require.NotNil(t, cfg)

	// Test files configuration
	assert.Equal(t, "https://127.0.0.1:1215/file", cfg.Files.BaseURL)
	assert.Equal(t, "/fs", cfg.Files.Path.FS)
	assert.Equal(t, "/client", cfg.Files.Path.Client)
	assert.Equal(t, "/cg", cfg.Files.Path.CG)
	assert.Equal(t, "read-write", cfg.Files.FS.Permission)
	assert.Equal(t, "nfs4.1", cfg.Files.FS.Protocol)
	assert.False(t, cfg.Files.FS.Exclusive)
	assert.Equal(t, []string{}, cfg.Files.FS.IgnoredClients)
	assert.Equal(t, 5*time.Minute, cfg.Files.CGTimeout())

	// Test DNS configuration
	assert.Equal(t, "https://127.0.0.1:1215/dns", cfg.DNS.BaseURL)
	assert.Equal(t, "/cname-entry", cfg.DNS.Path.CName)
	assert.Equal(t, "/zone", cfg.DNS.Path.Zone)

	// Test auth configuration
	assert.Equal(t, "https://127.0.0.1:1215/auth", cfg.Auth.BaseURL)
	assert.Equal(t, "the-secret", cfg.Auth.DefaultSecret)
	assert.Equal(t, []string{"files:read"}, cfg.Auth.Scopes["files_read"])
	assert.Equal(t, []string{"files:write"}, cfg.Auth.Scopes["files_write"])
	assert.Equal(t, 10, cfg.Auth.Timeout)
	assert.Equal(t, 1140, cfg.Auth.TTLSeconds)

	// Test cache configuration
	assert.Equal(t, 14400, cfg.Cache.TTLSeconds)
}

// TestLoadConfigSetsFSDefaults tests that a configuration file with no
// files.fs section still yields the package defaults.
func TestLoadConfigSetsFSDefaults(t *testing.T) {
	cfgFile := filepath.Join(t.TempDir(), "sgcp.yaml")
	require.NoError(t, os.WriteFile(cfgFile, []byte("files:\n  base_url: \"https://127.0.0.1:1215/file\"\n"), 0644))

	cfg, err := loadConfig(cfgFile)
	require.NoError(t, err)

	assert.Equal(t, FsDefaultPermission, cfg.Files.FS.Permission)
	assert.Equal(t, FsDefaultProtocol, cfg.Files.FS.Protocol)
	assert.False(t, cfg.Files.FS.Exclusive)
	assert.Equal(t, []string{}, cfg.Files.FS.IgnoredClients)
	assert.Equal(t, CGDefaultTimeout, cfg.Files.CGTimeout())
}

// TestCGTimeout tests the parsing of the files.cg.timeout setting
func TestCGTimeout(t *testing.T) {
	cases := map[string]struct {
		timeout  string
		expected time.Duration
	}{
		"unset":       {"", CGDefaultTimeout},
		"duration":    {"5m", 5 * time.Minute},
		"bare number": {"300", 300 * time.Second},
		"garbage":     {"not-a-duration", CGDefaultTimeout},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var files FilesConfig
			files.CG.Timeout = c.timeout
			assert.Equal(t, c.expected, files.CGTimeout())
		})
	}
}
