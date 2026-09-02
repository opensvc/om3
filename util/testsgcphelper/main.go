package testsgcphelper

import (
	"embed"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/rawconfig"
)

var (
	//go:embed text
	fs embed.FS

	disabledFlagLine = regexp.MustCompile(`(?m)^disabled_flag:.*$`)
)

// InstallConfig writes the sgcp test configuration in a directory of its own
// and returns its path.
//
// That directory is also the agent root for the duration of the test, so the
// ageing caches the sgcp code keeps and the locks it takes land there instead
// of in the node /var/lib/opensvc. The disabled flag is moved there too, as
// its mere path is stat'ed on every action. A test run thus needs no
// privileges, and leaves nothing behind on the machine it runs on.
func InstallConfig(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Cleanup(rawconfig.ReloadForTest(root))

	b, err := fs.ReadFile("text/config.yaml")
	require.NoError(t, err)
	b = disabledFlagLine.ReplaceAll(b, []byte("disabled_flag: "+filepath.Join(root, "sgcp_disabled")))

	cfgFile := filepath.Join(root, "sgcp.yaml")
	require.NoError(t, os.WriteFile(cfgFile, b, 0644))
	return cfgFile
}
