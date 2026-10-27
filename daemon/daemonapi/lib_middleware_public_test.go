package daemonapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/daemon/rbac"
)

// /metrics, which a prometheus scrapes, is public, and the detail pages,
// which name the objects and the usage of the node, are not.
func TestIsPublic(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		public       bool
	}{
		{http.MethodGet, "/metrics", true},
		{http.MethodGet, "/metrics/pg", false},
		{http.MethodGet, "/metrics/api", false},
		{http.MethodGet, "/ui/", true},
		{http.MethodGet, "/api/docs/index.html", true},
		{http.MethodGet, "/api/openapi", true},
		{http.MethodGet, "/api/auth/info", true},
		{http.MethodGet, "/api/node", false},
		{http.MethodPost, "/metrics", false},
	} {
		assert.Equal(t, tc.public, isPublic(tc.method, tc.path), "%s %s", tc.method, tc.path)
	}
}

// The detail metrics pages answer root alone.
func TestRequireRoot(t *testing.T) {
	handler := RequireRoot(func(c echo.Context) error { return c.String(http.StatusOK, "ok") })
	for _, tc := range []struct {
		grants rbac.Grants
		code   int
	}{
		{rbac.NewGrants("root"), http.StatusOK},
		{rbac.NewGrants("guest", "admin"), http.StatusForbidden},
		{rbac.Grants{}, http.StatusForbidden},
	} {
		e := echo.New()
		rec := httptest.NewRecorder()
		c := e.NewContext(httptest.NewRequest(http.MethodGet, "/metrics/pg", nil), rec)
		c.Set("grants", tc.grants)
		assert.NoError(t, handler(c))
		assert.Equal(t, tc.code, rec.Code, "%v", tc.grants)
	}
}
