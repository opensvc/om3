package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostObjectMonitor(w http.ResponseWriter, r *http.Request) {
	var (
		payload api.PostObjectMonitor
		update  instance.MonitorUpdate
		p       path.T
		err     error
	)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		WriteProblemf(w, http.StatusBadRequest, "Invalid body", "%s", err)
		return
	}
	p, err = path.Parse(payload.Path)
	if err != nil {
		WriteProblemf(w, http.StatusBadRequest, "Invalid body", "Error parsing path %s: %s", payload.Path, err)
		return
	}
	if payload.GlobalExpect != nil {
		i := instance.MonitorGlobalExpectValues[*payload.GlobalExpect]
		update.GlobalExpect = &i
	}
	if payload.LocalExpect != nil {
		i := instance.MonitorLocalExpectValues[*payload.LocalExpect]
		update.LocalExpect = &i
	}
	if payload.State != nil {
		i := instance.MonitorStateValues[*payload.State]
		update.State = &i
	}
	update.CandidateOrchestrationId = uuid.New()
	bus := pubsub.BusFromContext(r.Context())
	bus.Pub(&msgbus.SetInstanceMonitor{Path: p, Node: hostname.Hostname(), Value: update},
		pubsub.Label{"path", p.String()}, labelApi)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(api.MonitorUpdateQueued{
		OrchestrationId: update.CandidateOrchestrationId,
	})
	w.WriteHeader(http.StatusOK)
}
