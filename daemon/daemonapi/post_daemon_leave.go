package daemonapi

import (
	"net/http"

	"opensvc.com/opensvc/daemon/daemonauth"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/pubsub"
)

// PostDaemonLeave publishes msgbus.LeaveRequest{Node: node} with label node=<apinode>.
// It requires non empty params.Node
func (a *DaemonApi) PostDaemonLeave(w http.ResponseWriter, r *http.Request, params PostDaemonLeaveParams) {
	var (
		node string
	)
	log := daemonlogctx.Logger(r.Context()).With().Str("func", "PostDaemonLeave").Logger()

	grants := daemonauth.UserGrants(r)
	if !grants.HasAnyRole(daemonauth.RoleRoot, daemonauth.RoleLeave) {
		log.Info().Msg("not allowed, need at least 'root' or 'leave' grant")
		w.WriteHeader(http.StatusForbidden)
		return
	}

	node = params.Node
	// TODO verify is node value is a valid nodename
	if node == "" {
		log.Warn().Msgf("invalid node value: '%s'", node)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	bus := pubsub.BusFromContext(ctx)
	log.Info().Msgf("publish leave request for node %s", node)
	bus.Pub(msgbus.LeaveRequest{Node: node}, labelApi, labelNode)
	w.WriteHeader(http.StatusOK)
}
