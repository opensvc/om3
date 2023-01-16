package daemonapi

import (
	"net/http"

	"github.com/goccy/go-json"

	"opensvc.com/opensvc/core/node"
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
		Node:  hostname.Hostname(),
		Value: node.MonitorUpdate{},
	}
	if payload.LocalExpect != nil {
		validRequest = true
		i := node.MonitorLocalExpectValues[*payload.LocalExpect]
		cmd.Value.LocalExpect = &i
	}
	if payload.GlobalExpect != nil {
		validRequest = true
		i := node.MonitorGlobalExpectValues[*payload.GlobalExpect]
		cmd.Value.GlobalExpect = &i
	}
	if payload.State != nil {
		validRequest = true
		i := node.MonitorStateValues[*payload.State]
		cmd.Value.State = &i
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
