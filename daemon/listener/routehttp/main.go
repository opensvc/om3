/*
Package routehttp provides http mux

It defines routing for Opensvc listener daemons
*/
package routehttp

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/opensvc/om3/core/rawconfig"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo-contrib/pprof"
	"github.com/labstack/echo/v4"

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
	e := echo.New()
	pprof.Register(e)
	e.Use(mwProm)
	e.GET("/metrics", echoprometheus.NewHandler())
	e.File("/", filepath.Join(rawconfig.Paths.HTML, "index.html"))
	e.File("/auth-callback", filepath.Join(rawconfig.Paths.HTML, "index.html"))
	e.File("/index.js", filepath.Join(rawconfig.Paths.HTML, "index.js"))
	e.File("/favicon.ico", filepath.Join(rawconfig.Paths.HTML, "favicon.ico"))
	e.Use(daemonapi.LogMiddleware(ctx))
	e.Use(daemonapi.AuthMiddleware(ctx))
	e.Use(daemonapi.LogUserMiddleware(ctx))
	e.Use(daemonapi.LogRequestMiddleWare(ctx))
	api.RegisterHandlersWithBaseURL(e, daemonapi.New(ctx), "/api")
	api.RegisterHandlers(e, daemonapi.New(ctx))
	g := e.Group("/public/ui")
	if enableUI {
		g.Use(daemonapi.UIMiddleware(ctx))
	}

	return &T{mux: e}
}

// ServerHTTP implement http.Handler interface for T
func (t *T) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.mux.ServeHTTP(w, r)
}
