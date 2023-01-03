package daemonapi

import (
	"net/http"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

func (a *DaemonApi) PostNodeClear(w http.ResponseWriter, r *http.Request) {
	bus := pubsub.BusFromContext(r.Context())
	state := cluster.NodeMonitorStateIdle
	msg := msgbus.SetNodeMonitor{
		Node: hostname.Hostname(),
		Monitor: cluster.NodeMonitorUpdate{
			State: &state,
		},
	}
	bus.Pub(msg, labelApi)
	w.WriteHeader(http.StatusOK)
}
