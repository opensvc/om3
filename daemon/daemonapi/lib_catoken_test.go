package daemonapi

import (
	"os"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestToken returns a signed token carrying the ca claim. The signing key
// is irrelevant: caFromToken does not verify the signature, the node the token
// is sent to does.
func newTestToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("not-the-verifying-key"))
	require.NoError(t, err)
	return s
}

func TestCaFromToken(t *testing.T) {
	t.Run("returns the ca claim", func(t *testing.T) {
		chain := "-----BEGIN CERTIFICATE-----\nzzz\n-----END CERTIFICATE-----\n"
		tk := newTestToken(t, jwt.MapClaims{"ca": chain, "grant": []string{"join"}})
		ca, err := caFromToken(tk)
		require.NoError(t, err)
		assert.Equal(t, chain, string(ca))
	})

	t.Run("refuses a token without a usable ca claim", func(t *testing.T) {
		for name, tk := range map[string]string{
			"no ca claim":    newTestToken(t, jwt.MapClaims{"grant": []string{"root"}}),
			"empty ca claim": newTestToken(t, jwt.MapClaims{"ca": ""}),
			"not a token":    "not-a-token",
			"empty":          "",
		} {
			t.Run(name, func(t *testing.T) {
				_, err := caFromToken(tk)
				assert.Error(t, err)
			})
		}
	})
}

func TestTmpCertFile(t *testing.T) {
	b := []byte("-----BEGIN CERTIFICATE-----\nzzz\n-----END CERTIFICATE-----\n")
	name, err := tmpCertFile(b)
	require.NoError(t, err)
	defer func() { _ = os.Remove(name) }()

	got, err := os.ReadFile(name)
	require.NoError(t, err)
	assert.Equal(t, b, got)
}
