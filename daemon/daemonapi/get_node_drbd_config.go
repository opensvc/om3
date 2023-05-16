package daemonapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/handlers/handlerhelper"
)

func (a *DaemonApi) GetNodeDRBDConfig(w http.ResponseWriter, r *http.Request, params api.GetNodeDRBDConfigParams) {
	_, log := handlerhelper.GetWriteAndLog(w, r, "nodehandler.GetNodeDRBDConfig")
	log.Debug().Msg("starting")

	if params.Name == "" {
		log.Warn().Msgf("invalid file name: %s", params.Name)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	filename := fmt.Sprintf("/etc/drbd.d/%s.res", params.Name)
	resp := api.DRBDConfig{}

	if data, err := os.ReadFile(filename); err != nil {
		log.Info().Err(err).Msgf("Readfile %s (may be deleted)", filename)
		w.WriteHeader(http.StatusNotFound)
		return
	} else {
		resp.Data = data
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error().Err(err).Msg("json encode")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
