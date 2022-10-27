package daemonapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/relay"
)

func (a *DaemonApi) PostRelayMessage(w http.ResponseWriter, r *http.Request) {
	var (
		payload PostRelayMessage
		value   RelayMessage
	)
	log := daemonlogctx.Logger(r.Context()).With().Str("func", "PostRelayMessage").Logger()
	log.Debug().Msg("starting")

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	value.ClusterName = payload.ClusterName
	value.ClusterId = payload.ClusterId
	value.Nodename = payload.Nodename
	value.Msg = payload.Msg
	value.Updated = time.Now()
	value.Addr = r.RemoteAddr

	relay.Map.Store(payload.ClusterId, payload.Nodename, value)
	log.Info().Msgf("stored %s %s", payload.ClusterId, payload.Nodename)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "stored")
}
