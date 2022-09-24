package objecthandler

import (
	"encoding/json"
	"io"
	"net/http"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	PostObjectMonitor struct {
		Path         string `json:"path"`
		State        string `json:"state,omitempty"`
		LocalExpect  string `json:"local_expect,omitempty"`
		GlobalExpect string `json:"global_expect,omitempty"`
	}

	postObjectMonitorResponse struct {
		Status int               `json:"status"`
		Info   string            `json:"info"`
		Pub    PostObjectMonitor `json:"published_set_smon"`
	}
)

func PostMonitor(w http.ResponseWriter, r *http.Request) {
	var (
		p       path.T
		err     error
		payload = PostObjectMonitor{}
		smon    = instance.Monitor{}
	)
	write, log := handlerhelper.GetWriteAndLog(w, r, "objecthandler.PostMonitor")
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
	if p, err = path.Parse(payload.Path); err != nil {
		log.Error().Err(err).Msg("path.Parse")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	smon = instance.Monitor{
		GlobalExpect: payload.GlobalExpect,
		//LocalExpect:         payload.LocalExpect,
		//Status:              payload.State,
	}
	bus := pubsub.BusFromContext(r.Context())
	msg := msgbus.SetSmon{
		Path:    p,
		Node:    hostname.Hostname(),
		Monitor: smon,
	}
	msgbus.PubSetSmonUpdated(bus, p.String(), msg)

	response := postObjectMonitorResponse{Status: 0, Info: "daemon notified for monitor changed", Pub: payload}
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
