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

	"github.com/opensvc/om3/daemon/daemonauth"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/daemonlogctx"
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
			log = log.With().
				Str("request-uuid", reqUuid.String()).
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
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// TODO verify for alternate method for /public
			if c.Request().Method == http.MethodGet {
				if strings.HasPrefix(c.Path(), "/public") {
					extensions := daemonauth.NewGrants().Extensions()
					extensions.Add("strategy", "public")
					user := auth.NewUserInfo("nobody", "", nil, extensions)
					c.Set("user", user)
					return next(c)
				} else if c.Path() == "/metrics" {
					// TODO confirm no auth GET /metrics
					extensions := daemonauth.NewGrants().Extensions()
					extensions.Add("strategy", "metrics")
					user := auth.NewUserInfo("nobody", "", nil, extensions)
					c.Set("user", user)
					return next(c)
				}
			}
			log := LogHandler(c, "auth")
			req := c.Request()
			// serverAddr is used by AuthenticateRequest
			reqCtx := daemonctx.WithListenAddr(req.Context(), serverAddr)
			_, user, err := daemonauth.Strategies.AuthenticateRequest(req.WithContext(reqCtx))
			if err != nil {
				r := c.Request()
				log.Error().Err(err).Str("remote", r.RemoteAddr).Msg("auth")
				code := http.StatusUnauthorized
				return JSONProblem(c, code, http.StatusText(code), err.Error())
			}
			log.Debug().Msgf("user %s authenticated", user.GetUserName())
			c.Set("user", user)
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
				Str("auth-user", authUser.GetUserName()).
				Strs("auth-grant", extensions.Values("grant")).
				Str("auth-strategy", extensions.Get("strategy")).
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
				GetLogger(c).WithLevel(level).Msg("request")
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

func User(ctx echo.Context) auth.Info {
	return ctx.Get("user").(auth.Info)
}

func GrantsFromContext(ctx echo.Context) daemonauth.Grants {
	return daemonauth.NewGrants(User(ctx).GetExtensions()["grant"]...)
}

func Grants(user auth.Info) daemonauth.Grants {
	return daemonauth.NewGrants(user.GetExtensions()["grant"]...)
}

func LogHandler(c echo.Context, name string) *zerolog.Logger {
	l := c.Get("logger").(*zerolog.Logger).With().Str("func", name).Logger()
	return &l
}
