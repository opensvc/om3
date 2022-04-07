/*
	Package httpmux provides http mux

	It defines routing for Opensvc listener daemons

*/
package httpmux

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/handlers/daemonhandler"
	"opensvc.com/opensvc/daemon/handlers/objecthandler"
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
	mux.Use(daemonMiddleWare(ctx))
	mux.Use(daemondataMiddleWare(ctx))
	mux.Use(logMiddleWare(ctx))
	mux.Use(eventbusCmdCMiddleWare(ctx))
	mux.Post("/daemon_stop", daemonhandler.Stop)
	mux.Post("/object_status", objecthandler.PostStatus)
	mux.Get("/object_selector", objecthandler.GetSelector)
	mux.Get("/daemon_status", daemonhandler.GetStatus)
	mux.Mount("/daemon", t.newDaemonRouter())
	mux.Mount("/object", objecthandler.Router())

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
	r.Get("/eventsdemo", daemonhandler.Events)
	return r
}

func eventbusCmdCMiddleWare(parent context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := daemonctx.WithEventBusCmd(r.Context(), daemonctx.EventBusCmd(parent))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func logMiddleWare(parent context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqUuid := uuid.New()
			log := daemonctx.Logger(parent)
			ctx := daemonctx.WithLogger(r.Context(), log.With().Str("request-uuid", reqUuid.String()).Logger())
			ctx = daemonctx.WithUuid(ctx, reqUuid)
			log.Info().Str("METHOD", r.Method).Str("PATH", r.URL.Path).Msg("request")
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
			ctx := daemonctx.WithDaemonDataCmd(r.Context(), daemonctx.DaemonDataCmd(parent))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
