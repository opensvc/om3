package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) PostDaemonLogsControl(ctx echo.Context) error {
	if _, err := assertRoot(ctx); err != nil {
		return err
	}

	var (
		payload api.PostDaemonLogsControl
	)
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "error: %s", err)
	}
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
