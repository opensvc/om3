package daemonapi

import (
	"net/http"

	"github.com/goccy/go-json"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostNodeMonitor(w http.ResponseWriter, r *http.Request) {
	var (
		payload      api.PostNodeMonitor
		validRequest bool
	)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	value := node.MonitorUpdate{}
	if payload.LocalExpect != nil {
		validRequest = true
		i := node.MonitorLocalExpectValues[*payload.LocalExpect]
		value.LocalExpect = &i
	}
	if payload.GlobalExpect != nil {
		validRequest = true
		i := node.MonitorGlobalExpectValues[*payload.GlobalExpect]
		value.GlobalExpect = &i
	}
	if payload.State != nil {
		validRequest = true
		i := node.MonitorStateValues[*payload.State]
		value.State = &i
	}
	if !validRequest {
		sendError(w, http.StatusBadRequest, "need at least state, local_expect or global_expect")
		return
	}
	bus := pubsub.BusFromContext(r.Context())
	bus.Pub(&msgbus.SetNodeMonitor{Node: hostname.Hostname(), Value: value}, labelApi)
	response := api.ResponseInfoStatus{
		Info:   0,
		Status: "instance monitor change queued",
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("json encode")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
