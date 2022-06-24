package objecthandler

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	PostObjectMonitor struct {
		Path         string `json:"path"`
		State        string `json:"state,omitempty"`
		LocalExpect  string `json:"local_expect,omitempty"`
		GlobalExpect string `json:"global_expect,omitempty"`
	}

	postObjectMonitorResponse struct {
		status int    `json:"status"`
		info   string `json:"info"`
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
	if reqBody, err := ioutil.ReadAll(r.Body); err != nil {
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
		GlobalExpect:        payload.GlobalExpect,
		GlobalExpectUpdated: timestamp.Now(),
		LocalExpect:         payload.LocalExpect,
		Status:              payload.State,
	}
	evCmdC := daemonctx.DaemonPubSubCmd(r.Context())
	daemonps.PubSetSmonUpdated(evCmdC, p.String(), moncmd.SetSmon{
		Path:    p,
		Node:    hostname.Hostname(),
		Monitor: smon,
	})

	response := postObjectMonitorResponse{0, "instance monitor pushed pending ops"}
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
