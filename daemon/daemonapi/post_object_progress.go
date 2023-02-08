package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostObjectProgress(w http.ResponseWriter, r *http.Request) {
	var (
		payload   = PostObjectProgress{}
		p         path.T
		err       error
		isPartial bool
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
	state, ok := instance.MonitorStateValues[payload.State]
	if !ok {
		sendError(w, http.StatusBadRequest, "invalid state: "+payload.State)
		return
	}
	if payload.IsPartial != nil {
		isPartial = *payload.IsPartial
	}
	bus := pubsub.BusFromContext(r.Context())
	msg := msgbus.ProgressInstanceMonitor{
		Path:      p,
		Node:      hostname.Hostname(),
		SessionId: payload.SessionId,
		State:     state,
		IsPartial: isPartial,
	}
	bus.Pub(msg, pubsub.Label{"path", p.String()}, labelApi)
	w.WriteHeader(http.StatusOK)
}
