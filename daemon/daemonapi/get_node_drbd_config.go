package daemonapi

import (
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetNodeDRBDConfig(ctx echo.Context, nodename string, params api.GetNodeDRBDConfigParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	log := LogHandler(ctx, "GetNodeDRBDConfig")
	log.Tracef("starting")
	if params.Name == "" {
		log.Warnf("invalid file name: %s", params.Name)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid file name: %s", params.Name)
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.getLocalDRBDConfig(ctx, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeDRBDConfig(ctx.Request().Context(), nodename, &params)
	})
}

func (a *DaemonAPI) getLocalDRBDConfig(ctx echo.Context, params api.GetNodeDRBDConfigParams) error {
	filename := fmt.Sprintf("/etc/drbd.d/%s.res", params.Name)
	resp := api.DRBDConfig{}

	if data, err := os.ReadFile(filename); err != nil {
		log.Infof("read file %s (may be deleted): %s", filename, err)
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "ReadFile %s (may be deleted)", filename)
	} else {
		resp.Data = data
	}

	return ctx.JSON(http.StatusOK, resp)
}
