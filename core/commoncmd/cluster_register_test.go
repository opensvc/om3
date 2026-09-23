package commoncmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/env"
)

// writeCredentialFile returns the path of a file holding s.
func writeCredentialFile(t *testing.T, s string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "credential")
	require.NoError(t, os.WriteFile(path, []byte(s), 0o600))
	return path
}

// The credential is read before any node is asked to register, so that an
// operator naming a bad file is told about it once, instead of once per node.
func TestCollectorCredential(t *testing.T) {
	t.Run("reads the user and password from the file", func(t *testing.T) {
		user, password, err := CollectorCredential(writeCredentialFile(t, "collectoruser:s3cret\n"))
		require.NoError(t, err)
		assert.Equal(t, "collectoruser", user)
		assert.Equal(t, "s3cret", password)
	})

	t.Run("reads the environment when no file is named", func(t *testing.T) {
		t.Setenv(env.CollectorCredentialVar, "envuser:envsecret")
		user, password, err := CollectorCredential("")
		require.NoError(t, err)
		assert.Equal(t, "envuser", user)
		assert.Equal(t, "envsecret", password)
	})

	t.Run("the file wins over the environment", func(t *testing.T) {
		t.Setenv(env.CollectorCredentialVar, "envuser:envsecret")
		user, _, err := CollectorCredential(writeCredentialFile(t, "fileuser:filesecret"))
		require.NoError(t, err)
		assert.Equal(t, "fileuser", user)
	})

	t.Run("no credential at all is not an error", func(t *testing.T) {
		// The nodes then register with the id they already hold, as
		// "om node register" without a user does.
		user, password, err := CollectorCredential("")
		require.NoError(t, err)
		assert.Equal(t, "", user)
		assert.Equal(t, "", password)
	})

	t.Run("refuses a malformed credential", func(t *testing.T) {
		for name, value := range map[string]string{
			"no separator": "collectoruser",
			"no username":  ":s3cret",
			"no password":  "collectoruser:",
		} {
			t.Run(name, func(t *testing.T) {
				_, _, err := CollectorCredential(writeCredentialFile(t, value))
				assert.ErrorIs(t, err, ErrFlagInvalid)
			})
		}
	})

	t.Run("refuses a file it cannot read", func(t *testing.T) {
		_, _, err := CollectorCredential(filepath.Join(t.TempDir(), "absent"))
		assert.ErrorIs(t, err, ErrFlagInvalid)
	})
}
