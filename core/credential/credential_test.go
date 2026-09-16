package credential

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Run("splits a username and password pair", func(t *testing.T) {
		for name, tc := range map[string]struct {
			value        string
			wantUsername string
			wantPassword string
		}{
			"plain":                 {"root:secret", "root", "secret"},
			"password with a colon": {"root:a:b", "root", "a:b"},
			"password with spaces":  {"root:a b", "root", "a b"},
		} {
			t.Run(name, func(t *testing.T) {
				username, password, err := Parse(tc.value)
				require.NoError(t, err)
				assert.Equal(t, tc.wantUsername, username)
				assert.Equal(t, tc.wantPassword, password)
			})
		}
	})

	t.Run("refuses a pair with an empty half", func(t *testing.T) {
		for name, value := range map[string]string{
			"no colon":       "root",
			"empty":          "",
			"empty username": ":secret",
			"empty password": "root:",
			"colon alone":    ":",
		} {
			t.Run(name, func(t *testing.T) {
				_, _, err := Parse(value)
				assert.ErrorIs(t, err, ErrInvalid)
			})
		}
	})
}
