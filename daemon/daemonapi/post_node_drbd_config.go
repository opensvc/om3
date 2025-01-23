package daemonapi

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) PostNodeDRBDConfig(ctx echo.Context, nodename string, params api.PostNodeDRBDConfigParams) error {
	if _, err := assertRoot(ctx); err != nil {
		return err
	}
	payload := api.PostNodeDRBDConfigRequest{}
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}
	if a.localhost == nodename {
		return a.postLocalDRBDConfig(ctx, params, payload)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeDRBDConfig(ctx.Request().Context(), nodename, &params, payload)
	})
}

func (a *DaemonAPI) postLocalDRBDConfig(ctx echo.Context, params api.PostNodeDRBDConfigParams, payload api.PostNodeDRBDConfigRequest) error {
	if a, ok := pendingDRBDAllocations.get(payload.AllocationID); !ok || time.Now().After(a.ExpiredAt) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "drbd allocation expired: %#v", a)
	}
	if strings.Contains(params.Name, "..") || strings.HasPrefix(params.Name, "/") {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid body", "The 'name' parameter must be a basename.")
	}
	cf := fmt.Sprintf("/etc/drbd.d/%s.res", params.Name)
	if err := os.WriteFile(cf, payload.Data, 0644); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Error writing drbd res file", "%s", err)
	}
	return ctx.JSON(http.StatusOK, nil)
}
