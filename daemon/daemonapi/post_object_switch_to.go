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

func (a *DaemonApi) PostObjectSwitchTo(w http.ResponseWriter, r *http.Request) {
	var (
		payload = api.PostObjectSwitchTo{}
		value   = instance.MonitorUpdate{}
		p       path.T
		err     error
	)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		WriteProblem(w, http.StatusBadRequest, "Invalid Body", err.Error())
		return
	}
	p, err = path.Parse(payload.Path)
	if err != nil {
		WriteProblem(w, http.StatusBadRequest, "Invalid Body", "Invalid path: "+payload.Path)
		return
	}
	globalExpect := instance.MonitorGlobalExpectPlacedAt
	options := instance.MonitorGlobalExpectOptionsPlacedAt{}
	options.Destination = append(options.Destination, payload.Destination...)
	value = instance.MonitorUpdate{
		GlobalExpect:        &globalExpect,
		GlobalExpectOptions: options,
	}
	value.CandidateOrchestrationId = uuid.New()
	bus := pubsub.BusFromContext(r.Context())
	bus.Pub(&msgbus.SetInstanceMonitor{Path: p, Node: hostname.Hostname(), Value: value},
		pubsub.Label{"path", p.String()}, labelApi)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(api.MonitorUpdateQueued{
		OrchestrationId: value.CandidateOrchestrationId,
	})
	w.WriteHeader(http.StatusOK)
}
