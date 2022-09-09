package nodehandler

import (
	"encoding/json"
	"io"
	"net/http"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	PostNodeMonitor struct {
		State        string `json:"state,omitempty"`
		GlobalExpect string `json:"global_expect,omitempty"`
	}

	postNodeMonitorResponse struct {
		status int    `json:"status"`
		info   string `json:"info"`
	}
)

func PostMonitor(w http.ResponseWriter, r *http.Request) {
	var payload PostNodeMonitor
	write, log := handlerhelper.GetWriteAndLog(w, r, "nodehandler.PostMonitor")
	log.Debug().Msg("starting")
	if reqBody, err := io.ReadAll(r.Body); err != nil {
		log.Error().Err(err).Msg("read body request")
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if err := json.Unmarshal(reqBody, &payload); err != nil {
		log.Error().Err(err).Msg("request body unmarshal")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	cmd := daemonps.SetNmon{
		Node: hostname.Hostname(),
		Monitor: cluster.NodeMonitor{
			GlobalExpect: payload.GlobalExpect,
			Status:       payload.State,
		},
	}
	bus := pubsub.BusFromContext(r.Context())
	daemonps.PubSetNmon(bus, cmd)

	response := postNodeMonitorResponse{0, "instance monitor pushed pending ops"}
	b, err := json.Marshal(response)
	if err != nil {
		log.Error().Err(err).Msg("Marshal response")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err := write(b); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
