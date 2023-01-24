package daemonapi

import (
	"net/http"

	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/pubsub"
)

// PostDaemonJoin publishes msgbus.JoinRequest{Node: node} with label node=<apinode>.
// It requires non empty params.Node
func (a *DaemonApi) PostDaemonJoin(w http.ResponseWriter, r *http.Request, params PostDaemonJoinParams) {
	var (
		node string
	)
	log := daemonlogctx.Logger(r.Context()).With().Str("func", "PostDaemonJoin").Logger()

	node = params.Node
	// TODO verify is node value is a valid nodename
	if node == "" {
		log.Warn().Msgf("invalid node value: '%s'", node)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	bus := pubsub.BusFromContext(ctx)
	log.Info().Msgf("publish join request for node %s", node)
	bus.Pub(msgbus.JoinRequest{Node: node}, labelApi, labelNode)
	w.WriteHeader(http.StatusOK)
}
