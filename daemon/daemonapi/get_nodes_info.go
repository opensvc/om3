package daemonapi

import (
	"encoding/json"
	"net/http"

	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
)

func (p *DaemonApi) GetNodesInfo(w http.ResponseWriter, r *http.Request) {
	write, log := handlerhelper.GetWriteAndLog(w, r, "GetNodesInfo")
	log.Debug().Msg("starting")
	databus := daemondata.FromContext(r.Context())
	data := databus.GetNodesInfo()
	b, err := json.Marshal(data)
	if err != nil {
		log.Error().Err(err).Msg("marshal nodes info")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = write(b)
}
