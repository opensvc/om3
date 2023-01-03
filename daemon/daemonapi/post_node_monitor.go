package daemonapi

import (
	"net/http"

	"github.com/goccy/go-json"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

func (a *DaemonApi) PostNodeMonitor(w http.ResponseWriter, r *http.Request) {
	var (
		payload      PostNodeMonitor
		validRequest bool
	)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	cmd := msgbus.SetNodeMonitor{
		Node:    hostname.Hostname(),
		Monitor: cluster.NodeMonitorUpdate{},
	}
	if payload.LocalExpect != nil {
		validRequest = true
		i := cluster.NodeMonitorLocalExpectValues[*payload.LocalExpect]
		cmd.Monitor.LocalExpect = &i
	}
	if payload.GlobalExpect != nil {
		validRequest = true
		i := cluster.NodeMonitorGlobalExpectValues[*payload.GlobalExpect]
		cmd.Monitor.GlobalExpect = &i
	}
	if payload.State != nil {
		validRequest = true
		i := cluster.NodeMonitorStateValues[*payload.State]
		cmd.Monitor.State = &i
	}
	if !validRequest {
		sendError(w, http.StatusBadRequest, "need at least state, local_expect or global_expect")
		return
	}
	bus := pubsub.BusFromContext(r.Context())
	bus.Pub(cmd, labelApi)
	response := ResponseInfoStatus{
		Info:   0,
		Status: "instance monitor pushed pending ops",
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
