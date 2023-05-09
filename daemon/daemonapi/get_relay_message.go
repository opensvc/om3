package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/relay"
	"github.com/rs/zerolog/log"
)

func (a *DaemonApi) GetRelayMessage(w http.ResponseWriter, r *http.Request, params api.GetRelayMessageParams) {
	data := api.RelayMessages{}
	if params.ClusterId != nil && params.Nodename != nil {
		if msg, ok := relay.Map.Load(*params.ClusterId, *params.Nodename); !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		} else {
			data.Messages = []api.RelayMessage{msg.(api.RelayMessage)}
		}
	} else {
		l := relay.Map.List()
		data.Messages = make([]api.RelayMessage, len(l))
		for i, a := range l {
			data.Messages[i] = a.(api.RelayMessage)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("json encode")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
