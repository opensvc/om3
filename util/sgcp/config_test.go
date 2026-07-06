package sgcp

import (
	"embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	//go:embed text
	fs embed.FS
)

func Setup(t *testing.T) (cleanup func()) {
	t.Helper()
	tmpDir := t.TempDir()
	OrigDefaultConfigPath := DefaultConfigPath
	DefaultConfigPath = filepath.Join(tmpDir, "sgcp.yaml")
	cleanup = func() {
		DefaultConfigPath = OrigDefaultConfigPath
	}
	return cleanup
}

func InstallConfig(t *testing.T) {
	t.Helper()
	b, err := fs.ReadFile("text/config.yaml")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(DefaultConfigPath, b, 0755))
}

// TestGetScopes tests the GetScopes function
func TestGetScopes(t *testing.T) {
	defer Setup(t)()
	InstallConfig(t)
	cfg, err := LoadConfig()
	require.NoError(t, err)
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
	InstallConfig(t)

	config, err := LoadConfig()
	require.NoError(t, err)

	secret := config.GetDefaultSecret()
	assert.Equal(t, "the-secret", secret)
}

// TestDefaultConfigPath tests the default config path value
func TestDefaultConfigPath(t *testing.T) {
	assert.Equal(t, "/etc/om3/sgcp.yaml", DefaultConfigPath)
}

// TestGetConfig tests the global config getter when no config exists
func TestGetConfigWhenNotPresnt(t *testing.T) {
	defer Setup(t)()
	cfg, err := LoadConfig()
	assert.NotNil(t, err)
	assert.Nil(t, cfg)
}

// TestLoadConfig tests loading the configuration from a file
func TestLoadConfig(t *testing.T) {
	defer Setup(t)()
	InstallConfig(t)

	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Test files configuration
	assert.Equal(t, "https://127.0.0.1:1215/file", cfg.Files.BaseURL)
	assert.Equal(t, "/fs", cfg.Files.Path.FS)
	assert.Equal(t, "/client", cfg.Files.Path.Client)
	assert.Equal(t, "/cg", cfg.Files.Path.CG)

	// Test DNS configuration
	assert.Equal(t, "https://127.0.0.1:1215/dns", cfg.DNS.BaseURL)
	assert.Equal(t, "/alias", cfg.DNS.Path.Alias)
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
