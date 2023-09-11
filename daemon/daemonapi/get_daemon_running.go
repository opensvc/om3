package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (a *DaemonApi) GetDaemonRunning(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, true)
}
