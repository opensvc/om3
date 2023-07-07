/*
Package routehttp provides http mux

It defines routing for Opensvc listener daemons
*/
package routehttp

import (
	"context"
	"net/http"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo-contrib/pprof"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonapi"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/util/pubsub"
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
// when enableUi is true swagger-ui is serverd from /ui
func New(ctx context.Context, enableUi bool) *T {
	e := echo.New()
	pprof.Register(e)
	e.Use(mwProm)
	e.GET("/metrics", echoprometheus.NewHandler())
	e.Use(daemonapi.LogMiddleware(ctx))
	e.Use(daemonapi.AuthMiddleware(ctx))
	e.Use(daemonapi.LogUserMiddleware(ctx))
	e.Use(daemonapi.LogRequestMiddleWare(ctx))
	api.RegisterHandlers(e, &daemonapi.DaemonApi{
		Daemon:     daemonctx.Daemon(ctx),
		Daemondata: daemondata.FromContext(ctx),
		EventBus:   pubsub.BusFromContext(ctx),
		JWTcreator: ctx.Value("JWTCreator").(daemonapi.JWTCreater),
	})
	g := e.Group("/public/ui")
	if enableUi {
		g.Use(daemonapi.UiMiddleware(ctx))
	}

	// TODO convert to echo + openapi
	//mux.Get("/node_backlog", daemonhandler.GetNodeBacklog)
	//mux.Get("/node_log", daemonhandler.GetNodeLog)
	//mux.Get("/objects_backlog", objecthandler.GetObjectsBacklog)
	//mux.Get("/objects_log", objecthandler.GetObjectsLog)

	return &T{mux: e}
}

// ServerHTTP implement http.Handler interface for T
func (t *T) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.mux.ServeHTTP(w, r)
}
