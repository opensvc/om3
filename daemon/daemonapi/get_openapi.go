package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) GetSwagger(ctx echo.Context) error {
	swagger, err := api.GetSwagger()
	if err != nil {
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", err.Error())
	}
	return ctx.JSON(http.StatusOK, swagger)
}
