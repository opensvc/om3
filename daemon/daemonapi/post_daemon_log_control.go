package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) PostDaemonLogControl(ctx echo.Context, nodename string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	var payload api.LogControl
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "error: %s", err)
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.postLocalDaemonLogControl(ctx, payload)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostDaemonLogControl(ctx.Request().Context(), nodename, payload)
	})
}

func (a *DaemonAPI) postLocalDaemonLogControl(ctx echo.Context, payload api.LogControl) error {
	var level string
	if payload.Level != "none" {
		level = string(payload.Level)
	}
	newLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "Error parsing 'level': %s", err)
	}
	zerolog.SetGlobalLevel(newLevel)
	return JSONProblemf(ctx, http.StatusOK, "New log level", "%s", payload.Level)
}
