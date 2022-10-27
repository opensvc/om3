package daemonapi

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"opensvc.com/opensvc/daemon/daemonlogctx"
)

var (
	relayMap = &sync.Map{}
)

func makeRelayKey(clusterID, nodename string) string {
	return strings.Join([]string{clusterID, nodename}, "/")
}

func (a *DaemonApi) PostRelayMessage(w http.ResponseWriter, r *http.Request) {
	type (
		remoteAddrer interface {
			RemoteAddr() net.Addr
		}
	)
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

	key := makeRelayKey(payload.ClusterId, payload.Nodename)
	relayMap.Store(key, value)
	log.Info().Msgf("stored %s", key)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ResponseText(key))
}
