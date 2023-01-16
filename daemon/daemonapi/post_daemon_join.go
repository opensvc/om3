package daemonapi

import (
	"net/http"

	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/pubsub"
)

func (a *DaemonApi) PostDaemonJoin(w http.ResponseWriter, r *http.Request, params PostDaemonJoinParams) {
	var (
		newNode string
	)
	log := daemonlogctx.Logger(r.Context()).With().Str("func", "PostDaemonJoin").Logger()

	if params.Node != nil {
		newNode = *params.Node
	}
	// TODO verify is newNode value is a valid nodename
	if newNode == "" {
		log.Warn().Msgf("invalid node value: '%s'", newNode)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	bus := pubsub.BusFromContext(ctx)
	log.Info().Msgf("publish join request for node %s", newNode)
	bus.Pub(msgbus.JoinRequest{Node: newNode}, labelApi, labelPathCluster, labelNodeLocalhost)
	w.WriteHeader(http.StatusOK)
}
