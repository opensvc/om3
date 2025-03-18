package daemonapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) PostDaemonListenerLogControl(ctx echo.Context, nodename api.InPathNodeName, name api.InPathListenerName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	var (
		payload api.LogControl
	)
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "error: %s", err)
	}
	var level string
	if payload.Level != "none" {
		level = string(payload.Level)
	}
	_, err := zerolog.ParseLevel(level)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "Error parsing the level: %s", err)
	}
	return a.postDaemonSubAction(ctx, nodename, "log-level-"+level, fmt.Sprintf("lsnr-%s", name), func(c *client.T) (*http.Response, error) {
		return c.PostDaemonListenerLogControl(ctx.Request().Context(), nodename, name, payload)
	})
}
