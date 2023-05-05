package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/handlers/handlerhelper"
)

func (a *DaemonApi) GetNodesInfo(w http.ResponseWriter, r *http.Request) {
	_, log := handlerhelper.GetWriteAndLog(w, r, "GetNodesInfo")
	log.Debug().Msg("starting")
	data := node.GetNodesInfo()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("marshal nodes info")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
