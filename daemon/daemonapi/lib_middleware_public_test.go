package daemonapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// setPublicMetrics sets listener.public_metrics for the test.
func setPublicMetrics(t *testing.T, v bool) {
	t.Helper()
	if cluster.ConfigData.IsSet() {
		prev := cluster.ConfigData.Get()
		t.Cleanup(func() { cluster.ConfigData.Set(prev) })
	} else {
		t.Cleanup(cluster.InitData)
	}
	cfg := cluster.Config{}
	cfg.Listener.PublicMetrics = v
	cluster.ConfigData.Set(&cfg)
}

// The metrics pages are not public, /metrics included, where the webapp and
// the api documentation are.
func TestIsPublic(t *testing.T) {
	setPublicMetrics(t, false)
	for _, tc := range []struct {
		method, path string
		public       bool
	}{
		{http.MethodGet, "/metrics", false},
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

// listener.public_metrics opens /metrics alone, not the detail pages, nor the
// routes of the api.
func TestIsPublicWithPublicMetrics(t *testing.T) {
	setPublicMetrics(t, true)
	for _, tc := range []struct {
		method, path string
		public       bool
	}{
		{http.MethodGet, "/metrics", true},
		{http.MethodPost, "/metrics", false},
		{http.MethodGet, "/metrics/pg", false},
		{http.MethodGet, "/metrics/api", false},
		{http.MethodGet, "/api/node/name/n1/metrics", false},
	} {
		assert.Equal(t, tc.public, isPublic(tc.method, tc.path), "%s %s", tc.method, tc.path)
	}
}

// A page isPublic let through needs no grant.
func TestRequireScraperServesAPublicPage(t *testing.T) {
	handler := RequireScraper(func(c echo.Context) error { return c.String(http.StatusOK, "ok") })
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/metrics", nil), rec)
	c.Set("grants", rbac.Grants{})
	c.Set("strategy", daemonauth.StrategyPublic)
	assert.NoError(t, handler(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

// The metrics pages answer the scraper and root roles alone.
func TestRequireScraper(t *testing.T) {
	handler := RequireScraper(func(c echo.Context) error { return c.String(http.StatusOK, "ok") })
	for _, tc := range []struct {
		grants rbac.Grants
		code   int
	}{
		{rbac.NewGrants("root"), http.StatusOK},
		{rbac.NewGrants("scraper"), http.StatusOK},
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
