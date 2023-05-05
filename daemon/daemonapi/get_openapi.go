package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/handlers/handlerhelper"
)

func (a *DaemonApi) GetSwagger(w http.ResponseWriter, r *http.Request) {
	_, log := handlerhelper.GetWriteAndLog(w, r, "objecthandler.GetOpenapi")
	log.Debug().Msg("starting")

	swagger, err := api.GetSwagger()
	if err != nil {
		log.Info().Err(err).Msg("GetSwagger")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(swagger); err != nil {
		log.Error().Err(err).Msg("can't marshall schema")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
