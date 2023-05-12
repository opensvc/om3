package daemonapi

import (
	"net/http"

	"github.com/goccy/go-json"
	"github.com/google/uuid"

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
		update       node.MonitorUpdate
	)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		WriteProblem(w, http.StatusBadRequest, "Invalid Body", err.Error())
		return
	}
	if payload.LocalExpect != nil {
		validRequest = true
		i := node.MonitorLocalExpectValues[*payload.LocalExpect]
		update.LocalExpect = &i
	}
	if payload.GlobalExpect != nil {
		validRequest = true
		i := node.MonitorGlobalExpectValues[*payload.GlobalExpect]
		update.GlobalExpect = &i
	}
	if payload.State != nil {
		validRequest = true
		i := node.MonitorStateValues[*payload.State]
		update.State = &i
	}
	update.CandidateOrchestrationId = uuid.New()
	if !validRequest {
		WriteProblem(w, http.StatusBadRequest, "Invalid Body", "Need at least 'state', 'local_expect' or 'global_expect'")
		return
	}
	bus := pubsub.BusFromContext(r.Context())
	bus.Pub(&msgbus.SetNodeMonitor{Node: hostname.Hostname(), Value: update}, labelApi)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(api.MonitorUpdateQueued{
		OrchestrationId: update.CandidateOrchestrationId,
	})
	w.WriteHeader(http.StatusOK)
}
