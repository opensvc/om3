package daemonhandler

import (
	"encoding/json"
	"net/http"

	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondatactx"
)

func GetStatus(w http.ResponseWriter, r *http.Request) {
	funcName := "daemonhandler.GetStatus"
	log := daemonctx.Logger(r.Context()).With().Str("func", funcName).Logger()
	log.Debug().Msg("starting")
	databus := daemondatactx.DaemonData(r.Context())
	status := databus.GetStatus()
	b, err := json.Marshal(status)
	if err != nil {
		log.Error().Err(err).Msg("marshal status")
		w.WriteHeader(500)
		return
	}
	_, _ = write(w, r, funcName, b)
}
