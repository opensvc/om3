package daemonapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/handlers/handlerhelper"
	"github.com/opensvc/om3/util/drbd"
)

func (a *DaemonApi) GetNodeDrbdAllocation(w http.ResponseWriter, r *http.Request) {
	write, log := handlerhelper.GetWriteAndLog(w, r, "nodehandler.GetNodeDrbdAllocate")
	log.Debug().Msg("starting")

	resp := api.DrbdAllocation{
		ExpireAt: time.Now().Add(5 * time.Second),
	}

	digest, err := drbd.GetDigest()
	if err != nil {
		log.Error().Err(err).Msgf("get drbd dump digest")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if minor, err := digest.FreeMinor(); err != nil {
		log.Error().Err(err).Msgf("get free minor from drbd dump digest")
		w.WriteHeader(http.StatusNotFound)
		return
	} else {
		resp.Minor = minor
	}

	if port, err := digest.FreePort(); err != nil {
		log.Error().Err(err).Msgf("get free port from drbd dump digest")
		w.WriteHeader(http.StatusNotFound)
		return
	} else {
		resp.Port = port
	}

	b, err := json.Marshal(resp)
	if err != nil {
		log.Error().Err(err).Msg("marshal drbd allocation")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err := write(b); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
