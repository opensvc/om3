package sgcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installConfigWithDisabledFlag writes a minimal configuration naming flag as
// its disabled_flag, and makes it the configuration for the test duration.
func installConfigWithDisabledFlag(t *testing.T, flag string) {
	t.Helper()
	cfgFile := filepath.Join(t.TempDir(), "sgcp.yaml")
	require.NoError(t, os.WriteFile(cfgFile, []byte("disabled_flag: "+flag+"\n"), 0644))
	SetConfigForTest(cfgFile)
	t.Cleanup(func() { SetConfigForTest("") })
}

func TestIsDisabled(t *testing.T) {
	t.Run("no flag configured", func(t *testing.T) {
		installConfigWithDisabledFlag(t, "")

		disabled, err := IsDisabled(t.TempDir())
		require.NoError(t, err)
		assert.False(t, disabled)
	})

	t.Run("flag absent", func(t *testing.T) {
		dir := t.TempDir()
		installConfigWithDisabledFlag(t, filepath.Join(dir, "sgcp_disabled"))

		disabled, err := IsDisabled(dir)
		require.NoError(t, err)
		assert.False(t, disabled)
	})

	t.Run("flag present", func(t *testing.T) {
		dir := t.TempDir()
		flag := filepath.Join(dir, "sgcp_disabled")
		require.NoError(t, os.WriteFile(flag, nil, 0644))
		installConfigWithDisabledFlag(t, flag)

		disabled, err := IsDisabled(dir)
		require.NoError(t, err)
		assert.True(t, disabled)
	})

	t.Run("relative flag is resolved against dir", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "sgcp_disabled"), nil, 0644))
		installConfigWithDisabledFlag(t, "sgcp_disabled")

		disabled, err := IsDisabled(dir)
		require.NoError(t, err)
		assert.True(t, disabled)

		disabled, err = IsDisabled(t.TempDir())
		require.NoError(t, err)
		assert.False(t, disabled)
	})

	t.Run("an undecidable flag is an error, not a disable", func(t *testing.T) {
		dir := t.TempDir()
		notADir := filepath.Join(dir, "file")
		require.NoError(t, os.WriteFile(notADir, nil, 0644))
		installConfigWithDisabledFlag(t, filepath.Join(notADir, "sgcp_disabled"))

		disabled, err := IsDisabled(dir)
		require.Error(t, err)
		assert.False(t, disabled)
	})
}
