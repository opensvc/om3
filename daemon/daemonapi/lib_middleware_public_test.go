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
		{http.MethodGet, "", true},
		{http.MethodGet, "/ui", true},
		{http.MethodGet, "/api/docs", true},
		{http.MethodGet, "/api/docsfoo", false},
		{http.MethodGet, "/uifoo", false},
		{http.MethodGet, "/debug/pprof/", false},
		{http.MethodGet, "/debug/pprof/cmdline", false},
		{http.MethodPost, "/debug/pprof/symbol", false},
		{http.MethodGet, "/a/route/not/named", false},
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

// The routes under the prefix answer root alone, whatever their method, and
// the others are not concerned.
func TestRequireRootOn(t *testing.T) {
	handler := RequireRootOn("/debug/pprof")(func(c echo.Context) error { return c.String(http.StatusOK, "ok") })
	for _, tc := range []struct {
		method, path string
		grants       rbac.Grants
		code         int
	}{
		{http.MethodGet, "/debug/pprof/cmdline", rbac.NewGrants("root"), http.StatusOK},
		{http.MethodGet, "/debug/pprof/cmdline", rbac.NewGrants("scraper", "admin"), http.StatusForbidden},
		{http.MethodPost, "/debug/pprof/symbol", rbac.NewGrants("guest"), http.StatusForbidden},
		{http.MethodGet, "/debug/pprof/", rbac.Grants{}, http.StatusForbidden},
		{http.MethodGet, "/api/node", rbac.Grants{}, http.StatusOK},
	} {
		e := echo.New()
		rec := httptest.NewRecorder()
		c := e.NewContext(httptest.NewRequest(tc.method, tc.path, nil), rec)
		c.SetPath(tc.path)
		c.Set("grants", tc.grants)
		assert.NoError(t, handler(c))
		assert.Equal(t, tc.code, rec.Code, "%s %s %v", tc.method, tc.path, tc.grants)
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
