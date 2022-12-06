package daemonapi

import (
	"encoding/json"
	"net/http"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

func (a *DaemonApi) PostObjectMonitor(w http.ResponseWriter, r *http.Request) {
	var (
		payload = PostObjectMonitor{}
		smon    = instance.Monitor{}
		p       path.T
		err     error
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
	smon = instance.Monitor{}
	if payload.GlobalExpect != nil {
		smon.GlobalExpect = *payload.GlobalExpect
	}
	bus := pubsub.BusFromContext(r.Context())
	msg := msgbus.SetInstanceMonitor{
		Path:    p,
		Node:    hostname.Hostname(),
		Monitor: smon,
	}
	bus.Pub(msg, pubsub.Label{"path", p.String()}, labelApi)
	w.WriteHeader(http.StatusOK)
}
