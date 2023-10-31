package daemonapi

import (
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) GetNodeDRBDConfig(ctx echo.Context, params api.GetNodeDRBDConfigParams) error {
	log := LogHandler(ctx, "GetNodeDRBDConfig")
	log.Debugf("starting")

	if params.Name == "" {
		log.Warnf("invalid file name: %s", params.Name)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid file name: %s", params.Name)
	}

	filename := fmt.Sprintf("/etc/drbd.d/%s.res", params.Name)
	resp := api.DRBDConfig{}

	if data, err := os.ReadFile(filename); err != nil {
		log.Infof("ReadFile %s (may be deleted): %s", filename, err)
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "ReadFile %s (may be deleted)", filename)
	} else {
		resp.Data = data
	}

	return ctx.JSON(http.StatusOK, resp)
}
