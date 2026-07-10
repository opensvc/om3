package sgcp

import (
	"testing"

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
	assert.True(t, cfg.Cache.Enabled)
}
