/*
Package routehttp provides http mux

It defines routing for Opensvc listener daemons
*/
package routehttp

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo-contrib/pprof"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonapi"
)

type (
	T struct {
		mux *echo.Echo
	}
)

var (
	mwProm = echoprometheus.NewMiddleware("opensvc_api")
)

// New returns *T with log, rootDaemon
// it prepares middlewares and routes for Opensvc daemon listeners
// ui is handled from /ui/ with index.html
// when enableUI is true swagger-ui is served from /api/docs/
// metrics are served from /metrics
// profiling are served from /debug/pprof/
//
// redirections:
//
//	 "", "/", "/ui", "/ui/*" -> /ui/
//	/api/docs -> /api/docs/
func New(ctx context.Context, enableUI bool) *T {
	indexFilename := filepath.Join(rawconfig.Paths.HTML, "index.html")
	metricsURL := "/metrics"
	docPrefixURL := "/api/docs"
	docSpecURL := "/api/openapi"
	webappURL := "/ui"

	e := echo.New()
	pprof.Register(e)
	e.Use(mwProm)
	e.GET(metricsURL, echoprometheus.NewHandler())
	if enableUI {
		e.Group(docPrefixURL).Use(daemonapi.UIMiddleware(ctx, docPrefixURL, docSpecURL))
	}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			if path == webappURL || path == "" || path == "/" {
				return c.Redirect(http.StatusMovedPermanently, webappURL+"/")
			} else if path == docPrefixURL {
				return c.Redirect(http.StatusMovedPermanently, docPrefixURL+"/")
			} else if strings.HasPrefix(path, webappURL) {
				return c.File(indexFilename)
			}
			return next(c)
		}
	})

	e.Use(daemonapi.LogMiddleware(ctx))
	e.Use(daemonapi.AuthMiddleware(ctx))
	e.Use(daemonapi.LogUserMiddleware(ctx))
	e.Use(daemonapi.LogRequestMiddleWare(ctx))
	api.RegisterHandlers(e, daemonapi.New(ctx))

	return &T{mux: e}
}

// ServerHTTP implement http.Handler interface for T
func (t *T) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.mux.ServeHTTP(w, r)
}
