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
	log := getLogger(r, "PostObjectStatus")
	log.Debug().Msgf("starting")
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Warn().Err(err).Msgf("decode body")
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	p, err = path.Parse(payload.Path)
	if err != nil {
		log.Warn().Err(err).Msgf("can't parse path: %s, %s", payload.Path, payload)
		sendErrorf(w, http.StatusBadRequest, "invalid path %s", payload.Path)
		return
	}
	instanceStatus, err := postObjectStatusToInstanceStatus(p, payload)
	if err != nil {
		log.Warn().Err(err).Msgf("can't parse instance status %s", payload.Path)
		sendError(w, http.StatusBadRequest, "can't parse instance status")
		return
	}
	dataCmd := daemondata.BusFromContext(r.Context())
	if err := daemondata.SetInstanceStatus(dataCmd, p, *instanceStatus); err != nil {
		log.Warn().Err(err).Msgf("can't set instance status for %s", p)
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func postObjectStatusToInstanceStatus(p path.T, payload PostObjectStatus) (*instance.Status, error) {
	instanceStatus := instance.Status{
		Path:    p,
		Avail:   status.Parse(payload.Status.Avail),
		Overall: status.Parse(payload.Status.Overall),
		Updated: payload.Status.Updated,
		Frozen:  payload.Status.Frozen,
	}
	if prov, err := provisioned.NewFromString(string(payload.Status.Provisioned)); err != nil {
		return nil, err
	} else {
		instanceStatus.Provisioned = prov
	}
	if payload.Status.Optional != nil {
		instanceStatus.Optional = status.Parse(*payload.Status.Optional)
	}
	return &instanceStatus, nil
}
