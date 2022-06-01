package daemonhandler

import (
	"encoding/json"
	"net/http"

	"opensvc.com/opensvc/daemon/daemondatactx"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
)

func GetStatus(w http.ResponseWriter, r *http.Request) {
	write, log := handlerhelper.GetWriteAndLog(w, r, "daemonhandler.GetStatus")
	log.Debug().Msg("starting")
	databus := daemondatactx.DaemonData(r.Context())
	status := databus.GetStatus()
	b, err := json.Marshal(status)
	if err != nil {
		log.Error().Err(err).Msg("marshal status")
		w.WriteHeader(500)
		return
	}
	_, _ = write(b)
}
