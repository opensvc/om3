/*
Package routehttp provides http mux

It defines routing for Opensvc listener daemons
*/
package routehttp

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/shaj13/go-guardian/v2/auth"

	"opensvc.com/opensvc/daemon/daemonapi"
	"opensvc.com/opensvc/daemon/daemonauth"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/handlers/daemonhandler"
	"opensvc.com/opensvc/daemon/handlers/nodehandler"
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
	mux.Use(logMiddleWare(ctx))
	mux.Use(listenAddrMiddleWare(ctx))
	mux.Use(daemonauth.MiddleWare(ctx))
	mux.Use(logRequestMiddleWare(ctx))
	mux.Use(daemonMiddleWare(ctx))
	mux.Use(daemondataMiddleWare(ctx))
	mux.Use(eventbusCmdCMiddleWare(ctx))
	daemonapi.Register(mux)
	mux.Get("/auth/token", daemonauth.GetToken)
	mux.Get("/daemon_status", daemonhandler.GetStatus)
	mux.Post("/daemon_stop", daemonhandler.Stop)
	mux.Get("/objects_log", objecthandler.GetObjectsLog)
	mux.Get("/objects_backlog", objecthandler.GetObjectsBacklog)
	mux.Post("/node_monitor", nodehandler.PostMonitor)
	mux.Get("/node_log", daemonhandler.GetNodeLog)
	mux.Get("/node_backlog", daemonhandler.GetNodeBacklog)
	mux.Mount("/daemon", t.newDaemonRouter())
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
			addr := daemonctx.ListenAddr(parent)
			log := daemonlogctx.Logger(parent)
			log = log.With().Str("request-uuid", reqUuid.String()).Str("addr", addr).Logger()
			ctx := daemonlogctx.WithLogger(r.Context(), log)
			ctx = daemonctx.WithUuid(ctx, reqUuid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func logRequestMiddleWare(_ context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := auth.User(r).GetUserName()
			log := daemonlogctx.Logger(r.Context())
			if user != "" {
				log = log.With().Str("user", user).Logger()
				r = r.WithContext(daemonlogctx.WithLogger(r.Context(), log))
			}
			log.Info().Str("method", r.Method).Str("path", r.URL.Path).Msg("request")
			next.ServeHTTP(w, r)
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
