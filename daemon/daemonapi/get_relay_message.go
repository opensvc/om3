package daemonapi

import (
	"encoding/json"
	"net/http"

	"opensvc.com/opensvc/daemon/relay"
)

func (a *DaemonApi) GetRelayMessage(w http.ResponseWriter, r *http.Request, params GetRelayMessageParams) {
	data, ok := relay.Map.Load(params.ClusterId, params.Nodename)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(data)
}
