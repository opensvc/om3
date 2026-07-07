package testsgcphelper

import (
	"embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	//go:embed text
	fs embed.FS
)

func InstallConfig(t *testing.T) string {
	t.Helper()
	tmpCfgFile := filepath.Join(t.TempDir(), "sgcp.yaml")
	b, err := fs.ReadFile("text/config.yaml")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tmpCfgFile, b, 0755))
	return tmpCfgFile
}
