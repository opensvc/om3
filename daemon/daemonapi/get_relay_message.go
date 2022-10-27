package daemonapi

import (
	"encoding/json"
	"net/http"
)

func (a *DaemonApi) GetRelayMessage(w http.ResponseWriter, r *http.Request, params GetRelayMessageParams) {
	key := makeRelayKey(params.ClusterId, params.Nodename)
	data, ok := relayMap.Load(key)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(data)
}
