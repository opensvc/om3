package commoncmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretFromFileOrEnv(t *testing.T) {
	const envVar = "OSVC_TEST_SECRET"

	t.Run("reads the file, trimming the newline an editor adds", func(t *testing.T) {
		filename := filepath.Join(t.TempDir(), "secret")
		require.NoError(t, os.WriteFile(filename, []byte("s3cret\n"), 0600))

		s, err := SecretFromFileOrEnv(filename, envVar)
		require.NoError(t, err)
		assert.Equal(t, "s3cret", s)
	})

	t.Run("refuses an empty file", func(t *testing.T) {
		filename := filepath.Join(t.TempDir(), "secret")
		require.NoError(t, os.WriteFile(filename, []byte("  \n"), 0600))

		_, err := SecretFromFileOrEnv(filename, envVar)
		assert.ErrorContains(t, err, "is empty")
	})

	t.Run("refuses a file it can not read", func(t *testing.T) {
		_, err := SecretFromFileOrEnv(filepath.Join(t.TempDir(), "does-not-exist"), envVar)
		assert.Error(t, err)
	})

	t.Run("falls back to the environment", func(t *testing.T) {
		t.Setenv(envVar, "s3cret")

		s, err := SecretFromFileOrEnv("", envVar)
		require.NoError(t, err)
		assert.Equal(t, "s3cret", s)
	})

	t.Run("prefers the file over the environment", func(t *testing.T) {
		t.Setenv(envVar, "from-env")
		filename := filepath.Join(t.TempDir(), "secret")
		require.NoError(t, os.WriteFile(filename, []byte("from-file"), 0600))

		s, err := SecretFromFileOrEnv(filename, envVar)
		require.NoError(t, err)
		assert.Equal(t, "from-file", s)
	})

	t.Run("returns an empty string when there is no secret", func(t *testing.T) {
		t.Setenv(envVar, "")

		s, err := SecretFromFileOrEnv("", envVar)
		require.NoError(t, err)
		assert.Equal(t, "", s)
	})
}
