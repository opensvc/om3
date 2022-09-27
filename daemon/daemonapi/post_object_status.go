package daemonapi

import (
	"encoding/json"
	"net/http"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/daemon/daemondata"
)

func (d *DaemonApi) PostObjectStatus(w http.ResponseWriter, r *http.Request) {
	var (
		payload = PostObjectStatus{}
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
	instanceStatus := postObjectStatusToInstanceStatus(p, payload)
	dataCmd := daemondata.BusFromContext(r.Context())
	if err := daemondata.SetInstanceStatus(dataCmd, p, instanceStatus); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func postObjectStatusToInstanceStatus(p path.T, payload PostObjectStatus) instance.Status {
	instanceStatus := instance.Status{
		Path:        p,
		Avail:       status.Parse(payload.Status.Avail),
		Overall:     status.Parse(payload.Status.Overall),
		Updated:     payload.Status.Updated,
		Frozen:      payload.Status.Frozen,
		Provisioned: provisioned.FromBool(payload.Status.Provisioned),
	}
	if payload.Status.Optional != nil {
		instanceStatus.Optional = status.Parse(*payload.Status.Optional)
	}
	return instanceStatus
}
