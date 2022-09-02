/*
Package httpmux provides http mux

It defines routing for Opensvc listener daemons
*/
package routehttp

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"opensvc.com/opensvc/daemon/daemonauth"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/handlers/daemonhandler"
	"opensvc.com/opensvc/daemon/handlers/objecthandler"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	T struct {
		mux *chi.Mux
	}
)

// New returns *T with log, rootDaemon
// it prepares middlewares and routes for Opensvc daemon listeners
func New(ctx context.Context) *T {
	t := &T{}
	mux := chi.NewRouter()
	mux.Use(listenAddrMiddleWare(ctx))
	mux.Use(daemonauth.MiddleWare(ctx))
	mux.Use(daemonMiddleWare(ctx))
	mux.Use(daemondataMiddleWare(ctx))
	mux.Use(logMiddleWare(ctx))
	mux.Use(eventbusCmdCMiddleWare(ctx))
	mux.Get("/auth/token", daemonauth.GetToken)
	mux.Get("/daemon_status", daemonhandler.GetStatus)
	mux.Post("/daemon_stop", daemonhandler.Stop)
	mux.Post("/object_monitor", objecthandler.PostMonitor)
	mux.Post("/object_status", objecthandler.PostStatus)
	mux.Get("/object_selector", objecthandler.GetSelector)
	mux.Get("/object_config", objecthandler.GetConfig)
	mux.Get("/object_config_file", objecthandler.GetConfigFile)
	mux.Get("/objects_log", objecthandler.GetObjectsLog)
	mux.Get("/objects_backlog", objecthandler.GetObjectsBacklog)
	mux.Get("/node_log", daemonhandler.GetNodeLog)
	mux.Get("/node_backlog", daemonhandler.GetNodeBacklog)
	mux.Mount("/daemon", t.newDaemonRouter())
	mux.Mount("/object", objecthandler.Router())
	mux.Mount("/debug", middleware.Profiler())

	t.mux = mux
	return t
}

// ServerHTTP implement http.Handler interface for T
func (t *T) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.mux.ServeHTTP(w, r)
}

func (t *T) newDaemonRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Get("/running", daemonhandler.Running)
	r.Get("/status", daemonhandler.GetStatus)
	r.Post("/stop", daemonhandler.Stop)
	r.Get("/events", daemonhandler.Events)
	return r
}

func eventbusCmdCMiddleWare(parent context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := pubsub.ContextWithBus(r.Context(), pubsub.BusFromContext(parent)) // Why ?
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func logMiddleWare(parent context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqUuid := uuid.New()
			log := daemonlogctx.Logger(parent)
			ctx := daemonlogctx.WithLogger(r.Context(), log.With().Str("request-uuid", reqUuid.String()).Logger())
			ctx = daemonctx.WithUuid(ctx, reqUuid)
			log.Info().Str("method", r.Method).Str("path", r.URL.Path).Msg("request")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func daemonMiddleWare(parent context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := daemonctx.WithDaemon(r.Context(), daemonctx.Daemon(parent))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func daemondataMiddleWare(parent context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := daemondata.ContextWithBus(r.Context(), daemondata.BusFromContext(parent))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// listenAddrMiddleWare adds the listen addr to the request context, for use by the ux auth middleware
func listenAddrMiddleWare(parent context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			addr := daemonctx.ListenAddr(parent)
			ctx := daemonctx.WithListenAddr(r.Context(), addr)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
