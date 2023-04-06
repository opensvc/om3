package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostObjectMonitor(w http.ResponseWriter, r *http.Request) {
	var (
		payload     = PostObjectMonitor{}
		instMonitor = instance.MonitorUpdate{}
		p           path.T
		err         error
	)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	p, err = path.Parse(payload.Path)
	if err != nil {
		sendError(w, http.StatusBadRequest, "invalid path: "+payload.Path)
		return
	}
	if payload.GlobalExpect != nil {
		i := instance.MonitorGlobalExpectValues[*payload.GlobalExpect]
		instMonitor.GlobalExpect = &i
	}
	if payload.LocalExpect != nil {
		i := instance.MonitorLocalExpectValues[*payload.LocalExpect]
		instMonitor.LocalExpect = &i
	}
	if payload.State != nil {
		i := instance.MonitorStateValues[*payload.State]
		instMonitor.State = &i
	}
	orchestrationId := uuid.New().String()
	instMonitor.CandidateOrchestrationId = orchestrationId
	bus := pubsub.BusFromContext(r.Context())
	bus.Pub(&msgbus.SetInstanceMonitor{Path: p, Node: hostname.Hostname(), Value: instMonitor},
		pubsub.Label{"path", p.String()}, labelApi)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(orchestrationId)
}
