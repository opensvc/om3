package daemonapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/handlers/handlerhelper"
)

func (a *DaemonApi) GetNodeDrbdConfig(w http.ResponseWriter, r *http.Request, params api.GetNodeDrbdConfigParams) {
	write, log := handlerhelper.GetWriteAndLog(w, r, "nodehandler.GetNodeDrbdConfig")
	log.Debug().Msg("starting")

	if params.Name == "" {
		log.Warn().Msgf("invalid file name: %s", params.Name)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	filename := fmt.Sprintf("/etc/drbd.d/%s.res", params.Name)
	resp := api.ResponseGetNodeDrbdConfig{}

	if data, err := os.ReadFile(filename); err != nil {
		log.Info().Err(err).Msgf("Readfile %s (may be deleted)", filename)
		w.WriteHeader(http.StatusNotFound)
		return
	} else {
		resp.Data = data
	}

	b, err := json.Marshal(resp)
	if err != nil {
		log.Error().Err(err).Msgf("marshal response error %s", filename)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err := write(b); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
