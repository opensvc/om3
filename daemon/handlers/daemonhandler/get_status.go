package daemonhandler

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
)

type getDaemonStatus struct {
	Namespace string
	Selector  string
	Relatives bool
}

func GetStatus(w http.ResponseWriter, r *http.Request) {
	write, log := handlerhelper.GetWriteAndLog(w, r, "daemonhandler.GetStatus")
	log.Debug().Msg("starting")

	payload := getDaemonStatus{}
	if reqBody, err := ioutil.ReadAll(r.Body); err != nil {
		log.Error().Err(err).Msg("read body request")
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if len(reqBody) == 0 {
		// pass
	} else if err := json.Unmarshal(reqBody, &payload); err != nil {
		log.Error().Err(err).Msg("request body unmarshal")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	databus := daemondata.FromContext(r.Context())
	status := databus.GetStatus().
		WithSelector(payload.Selector).
		WithNamespace(payload.Namespace)
		// TODO: WithRelatives()

	b, err := json.Marshal(status)
	if err != nil {
		log.Error().Err(err).Msg("marshal status")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = write(b)
}
