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
	log.Debug().Msg("starting")

	if params.Name == "" {
		log.Warn().Msgf("invalid file name: %s", params.Name)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid file name: %s", params.Name)
	}

	filename := fmt.Sprintf("/etc/drbd.d/%s.res", params.Name)
	resp := api.DRBDConfig{}

	if data, err := os.ReadFile(filename); err != nil {
		log.Info().Err(err).Msgf("Readfile %s (may be deleted)", filename)
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "Readfile %s (may be deleted)", filename)
	} else {
		resp.Data = data
	}

	return ctx.JSON(http.StatusOK, resp)
}
