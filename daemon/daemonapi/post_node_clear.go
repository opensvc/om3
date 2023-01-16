package daemonapi

import (
	"net/http"

	"opensvc.com/opensvc/core/node"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

func (a *DaemonApi) PostNodeClear(w http.ResponseWriter, r *http.Request) {
	bus := pubsub.BusFromContext(r.Context())
	state := node.MonitorStateIdle
	msg := msgbus.SetNodeMonitor{
		Node: hostname.Hostname(),
		Value: node.MonitorUpdate{
			State: &state,
		},
	}
	bus.Pub(msg, labelApi)
	w.WriteHeader(http.StatusOK)
}
