package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/handlers/handlerhelper"
)

func (a *DaemonApi) GetSwagger(w http.ResponseWriter, r *http.Request) {
	var b []byte
	var err error
	write, log := handlerhelper.GetWriteAndLog(w, r, "objecthandler.GetOpenapi")
	log.Debug().Msg("starting")

	swagger, err := api.GetSwagger()
	if err != nil {
		log.Info().Err(err).Msg("GetSwagger")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	b, err = json.Marshal(swagger)
	if err != nil {
		log.Error().Err(err).Msg("can't marshall schema")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err := write(b); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
