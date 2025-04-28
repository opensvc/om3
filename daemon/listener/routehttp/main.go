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
// when enableUI is true swagger-ui is serverd from /ui
func New(ctx context.Context, enableUI bool) *T {
	indexFilename := filepath.Join(rawconfig.Paths.HTML, "index.html")
	metricsURL := "/metrics"
	docPrefixURL := "/doc"
	docSpecURL := "/api/openapi"

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
			if strings.HasPrefix(path, "/api/") || path == metricsURL || strings.HasPrefix(path, docPrefixURL+"/") {
				return next(c)
			}
			return c.File(indexFilename)
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
