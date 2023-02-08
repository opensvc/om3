package daemonapi

import (
	"net/http"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
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
