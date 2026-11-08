package daemonapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/util/plog"
)

// refusingStrategy refuses every request with err.
type refusingStrategy struct {
	err error
}

func (s refusingStrategy) Authenticate(context.Context, *http.Request) (*daemonauth.Info, error) {
	return nil, s.err
}

func setStrategies(t *testing.T, u daemonauth.Union) {
	t.Helper()
	daemonauth.Strategies.Mutex.Lock()
	prev := daemonauth.Strategies.Value
	daemonauth.Strategies.Value = u
	daemonauth.Strategies.Mutex.Unlock()
	t.Cleanup(func() {
		daemonauth.Strategies.Mutex.Lock()
		daemonauth.Strategies.Value = prev
		daemonauth.Strategies.Mutex.Unlock()
	})
}

// The 401 of the auth middleware carries the WWW-Authenticate challenge of
// RFC 6750, which tells a node its token was spent before the peer read it.
func TestAuthMiddlewareChallenge(t *testing.T) {
	setPublicMetrics(t, false)
	for name, tc := range map[string]struct {
		authorization string
		err           error
		expected      string
	}{
		"no credentials": {
			err:      errors.New("strategies/jwt: request has no bearer token"),
			expected: "Bearer",
		},
		"an expired token": {
			authorization: "Bearer tk",
			err:           fmt.Errorf("strategies/jwt: %w", fmt.Errorf("%w: %w", jwt.ErrTokenInvalidClaims, jwt.ErrTokenExpired)),
			expected:      `Bearer error="invalid_token", error_description="the token is expired"`,
		},
		"a refused token": {
			authorization: "Bearer tk",
			err:           fmt.Errorf("strategies/jwt: %w", jwt.ErrTokenSignatureInvalid),
			expected:      `Bearer error="invalid_token"`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			setStrategies(t, daemonauth.Union{refusingStrategy{err: tc.err}})
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/api/node", nil)
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/api/node")
			c.Set("logger", plog.NewDefaultLogger())
			next := func(c echo.Context) error {
				return c.NoContent(http.StatusOK)
			}
			require.NoError(t, AuthMiddleware(context.Background())(next)(c))
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.Equal(t, tc.expected, rec.Header().Get("WWW-Authenticate"))
		})
	}
}
