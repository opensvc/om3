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
	"github.com/rs/zerolog"
	"github.com/shaj13/go-guardian/v2/auth"

	"opensvc.com/opensvc/daemon/daemonapi"
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

var (
	// logRequestLevelPerPath defines logRequestMiddleWare log level per path.
	// The default value is LevelInfo
	logRequestLevelPerPath = map[string]zerolog.Level{
		"/relay/message": zerolog.DebugLevel,
	}
)

// New returns *T with log, rootDaemon
// it prepares middlewares and routes for Opensvc daemon listeners
// when enableUi is true swagger-ui is serverd from /ui
func New(ctx context.Context, enableUi bool) *T {
	t := &T{}
	mux := chi.NewRouter()
	// cors not required since /ui is served from swagger-ui
	//mux.Use(cors.Handler(cors.Options{
	//	// TODO update AllowedOrigins, and verify other settings
	//	AllowedOrigins:     []string{"https://editor.swagger.io", "https://editor-next.swagger.io"},
	//	AllowedMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	//	ExposedHeaders:     []string{"Link"},
	//	AllowedHeaders:     []string{"Authorization"},
	//	AllowCredentials:   false,
	//	MaxAge:             300, // Maximum value not ignored by any of major browsers
	//	OptionsPassthrough: false,
	//	Debug:              true,
	//}))
	mux.Use(logMiddleWare(ctx))
	mux.Use(listenAddrMiddleWare(ctx))
	mux.Use(daemonauth.MiddleWare(ctx))
	mux.Use(logUserMiddleWare(ctx))
	mux.Use(logRequestMiddleWare(ctx))
	mux.Use(daemonMiddleWare(ctx))
	mux.Use(daemondataMiddleWare(ctx))
	mux.Use(eventbusCmdCMiddleWare(ctx))
	daemonapi.Register(mux, enableUi)
	mux.Get("/objects_log", objecthandler.GetObjectsLog)
	mux.Get("/objects_backlog", objecthandler.GetObjectsBacklog)
	mux.Get("/node_log", daemonhandler.GetNodeLog)
	mux.Get("/node_backlog", daemonhandler.GetNodeBacklog)
	mux.Mount("/debug", middleware.Profiler())

	t.mux = mux
	return t
}

// ServerHTTP implement http.Handler interface for T
func (t *T) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.mux.ServeHTTP(w, r)
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
			log = log.With().
				Str("request-uuid", reqUuid.String()).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote", r.RemoteAddr).
				Str("addr", addr).Logger()
			ctx := daemonlogctx.WithLogger(r.Context(), log)
			ctx = daemonctx.WithUuid(ctx, reqUuid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func logUserMiddleWare(_ context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := auth.User(r).GetUserName()
			if user != "" {
				log := daemonlogctx.Logger(r.Context()).With().Str("user", user).Logger()
				r = r.WithContext(daemonlogctx.WithLogger(r.Context(), log))
			}
			next.ServeHTTP(w, r)
		})
	}
}

func logRequestMiddleWare(_ context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			level := zerolog.InfoLevel
			if l, ok := logRequestLevelPerPath[r.URL.Path]; ok {
				level = l
			}
			if level != zerolog.NoLevel {
				log := daemonlogctx.Logger(r.Context())
				log.WithLevel(level).Msg("request")
			}
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
