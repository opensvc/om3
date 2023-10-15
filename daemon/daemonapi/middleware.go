package daemonapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/allenai/go-swaggerui"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/shaj13/go-guardian/v2/auth"

	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/daemonlogctx"
	"github.com/opensvc/om3/daemon/rbac"
)

type (
	Strategier interface {
		AuthenticateRequest(r *http.Request) (auth.Strategy, auth.Info, error)
	}
)

var (
	// logRequestLevelPerPath defines logRequestMiddleWare log level per path.
	// The default value is LevelInfo
	logRequestLevelPerPath = map[string]zerolog.Level{
		"/metrics":        zerolog.DebugLevel,
		"/public/openapi": zerolog.DebugLevel,
		"/public/ui/*":    zerolog.DebugLevel,
		"/relay/message":  zerolog.DebugLevel,
	}
)

func LogMiddleware(parent context.Context) echo.MiddlewareFunc {
	addr := daemonctx.ListenAddr(parent)
	log := daemonlogctx.Logger(parent)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			reqUuid := uuid.New()
			r := c.Request()
			log := log.With().
				Str("request_uuid", reqUuid.String()).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote", r.RemoteAddr).
				Str("addr", addr).Logger()
			c.Set("logger", &log)
			c.Set("uuid", reqUuid)
			return next(c)
		}
	}
}

func AuthMiddleware(parent context.Context) echo.MiddlewareFunc {
	serverAddr := daemonctx.ListenAddr(parent)
	strategies := parent.Value("authStrategies").(Strategier)
	newExtensions := func(strategy string) *auth.Extensions {
		return &auth.Extensions{"strategy": []string{strategy}}
	}

	isPublic := func(c echo.Context) bool {
		if c.Request().Method != http.MethodGet {
			return false
		}
		usrPath := c.Path()
		// TODO confirm no auth GET /metrics
		return strings.HasPrefix(usrPath, "/public") || strings.HasPrefix(usrPath, "/metrics")
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// TODO verify for alternate method for /public, /metrics
			if isPublic(c) {
				user := auth.NewUserInfo("nobody", "", nil, *newExtensions("public"))
				c.Set("user", user)
				c.Set("grants", rbac.Grants{})
				return next(c)
			}
			log := LogHandler(c, "auth")
			req := c.Request()
			// serverAddr is used by AuthenticateRequest
			reqCtx := daemonctx.WithListenAddr(req.Context(), serverAddr)
			_, user, err := strategies.AuthenticateRequest(req.WithContext(reqCtx))
			if err != nil {
				r := c.Request()
				log.Error().Err(err).Str("remote", r.RemoteAddr).Msgf("daemon: api: error authenticating request from %s: %s", r.RemoteAddr, err)
				code := http.StatusUnauthorized
				return JSONProblem(c, code, http.StatusText(code), err.Error())
			}
			log.Debug().Msgf("daemon: api: user %s authenticated", user.GetUserName())
			c.Set("user", user)
			c.Set("grants", rbac.NewGrants(user.GetExtensions()["grant"]...))
			return next(c)
		}
	}
}

func LogUserMiddleware(parent context.Context) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authUser := c.Get("user").(auth.Info)
			extensions := authUser.GetExtensions()
			log := c.Get("logger").(*zerolog.Logger).With().
				Str("auth_user", authUser.GetUserName()).
				Strs("auth_grant", extensions.Values("grant")).
				Str("auth_strategy", extensions.Get("strategy")).
				Logger()
			c.Set("logger", &log)
			return next(c)
		}
	}
}

func LogRequestMiddleWare(parent context.Context) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			level := zerolog.InfoLevel
			if l, ok := logRequestLevelPerPath[c.Path()]; ok {
				level = l
			}
			if level != zerolog.NoLevel {
				GetLogger(c).WithLevel(level).Msgf("daemon: api: new request %s: %s %s from user %s address %s", c.Get("uuid"), c.Request().Method, c.Path(), userFromContext(c).GetUserName(), c.Request().RemoteAddr)
			}
			return next(c)
		}
	}
}

func UiMiddleware(_ context.Context) echo.MiddlewareFunc {
	uiHandler := http.StripPrefix("/public/ui", swaggerui.Handler("/public/openapi"))
	echoUi := echo.WrapHandler(uiHandler)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return echoUi(c)
		}
	}
}

func GetLogger(c echo.Context) *zerolog.Logger {
	return c.Get("logger").(*zerolog.Logger)
}

// userFromContext returns the logged-in userFromContext information stored in the request context.
func userFromContext(ctx echo.Context) auth.Info {
	return ctx.Get("user").(auth.Info)
}

func grantsFromContext(ctx echo.Context) rbac.Grants {
	return ctx.Get("grants").(rbac.Grants)
}

func LogHandler(c echo.Context, name string) *zerolog.Logger {
	l := c.Get("logger").(*zerolog.Logger).With().Str("func", name).Logger()
	return &l
}
