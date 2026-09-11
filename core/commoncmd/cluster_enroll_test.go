package commoncmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/env"
)

func TestCmdClusterEnrollToken(t *testing.T) {
	writeTokenFile := func(t *testing.T, s string) string {
		t.Helper()
		name := filepath.Join(t.TempDir(), "token")
		require.NoError(t, os.WriteFile(name, []byte(s), 0600))
		return name
	}

	t.Run("reads the token from --token", func(t *testing.T) {
		t.Setenv(env.JoinTokenVar, "")
		c := CmdClusterEnroll{Token: "from-flag"}
		got, err := c.token()
		require.NoError(t, err)
		assert.Equal(t, "from-flag", got)
	})

	t.Run("reads the token from --token-file, trimmed", func(t *testing.T) {
		t.Setenv(env.JoinTokenVar, "")
		c := CmdClusterEnroll{TokenFile: writeTokenFile(t, "  from-file\n")}
		got, err := c.token()
		require.NoError(t, err)
		assert.Equal(t, "from-file", got)
	})

	t.Run("prefers --token-file over the environment", func(t *testing.T) {
		t.Setenv(env.JoinTokenVar, "from-env")
		c := CmdClusterEnroll{TokenFile: writeTokenFile(t, "from-file")}
		got, err := c.token()
		require.NoError(t, err)
		assert.Equal(t, "from-file", got)
	})

	t.Run("falls back to the environment", func(t *testing.T) {
		t.Setenv(env.JoinTokenVar, "from-env")
		c := CmdClusterEnroll{}
		got, err := c.token()
		require.NoError(t, err)
		assert.Equal(t, "from-env", got)
	})

	t.Run("refuses --token with --token-file", func(t *testing.T) {
		t.Setenv(env.JoinTokenVar, "")
		c := CmdClusterEnroll{Token: "from-flag", TokenFile: writeTokenFile(t, "from-file")}
		_, err := c.token()
		assert.ErrorIs(t, err, ErrFlagInvalid)
	})

	t.Run("refuses an empty --token-file", func(t *testing.T) {
		t.Setenv(env.JoinTokenVar, "")
		c := CmdClusterEnroll{TokenFile: writeTokenFile(t, "\n  \n")}
		_, err := c.token()
		assert.ErrorIs(t, err, ErrFlagInvalid)
	})

	t.Run("refuses an unreadable --token-file", func(t *testing.T) {
		t.Setenv(env.JoinTokenVar, "")
		c := CmdClusterEnroll{TokenFile: filepath.Join(t.TempDir(), "absent")}
		_, err := c.token()
		assert.ErrorIs(t, err, ErrFlagInvalid)
	})

	t.Run("refuses no token at all", func(t *testing.T) {
		t.Setenv(env.JoinTokenVar, "")
		c := CmdClusterEnroll{}
		_, err := c.token()
		assert.ErrorIs(t, err, ErrFlagInvalid)
	})
}
