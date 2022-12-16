package daemonapi

import (
	"net/http"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

func (a *DaemonApi) PostNodeClear(w http.ResponseWriter, r *http.Request) {
	var (
		nmon = cluster.NodeMonitor{}
	)
	nmon = cluster.NodeMonitor{
		State: cluster.NodeMonitorStateIdle,
	}
	bus := pubsub.BusFromContext(r.Context())
	msg := msgbus.SetNodeMonitor{
		Node:    hostname.Hostname(),
		Monitor: nmon,
	}
	bus.Pub(msg, labelApi)
	w.WriteHeader(http.StatusOK)
}
