package objecthandler

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
)

type (
	GetObjectSelector struct {
		ObjectSelector string `json:"selector"`
	}
)

func GetSelector(w http.ResponseWriter, r *http.Request) {
	write, log := handlerhelper.GetWriteAndLog(w, r, "objecthandler.GetSelector")
	log.Debug().Msg("starting")
	payload := GetObjectSelector{}
	if reqBody, err := ioutil.ReadAll(r.Body); err != nil {
		log.Error().Err(err).Msg("read body request")
		w.WriteHeader(500)
		return
	} else if err := json.Unmarshal(reqBody, &payload); err != nil {
		log.Error().Err(err).Msg("request body unmarshal")
		w.WriteHeader(500)
		return
	}
	selector := payload.ObjectSelector
	daemonData := daemondata.FromContext(r.Context())
	paths := daemonData.GetServiceNames()
	matchedPaths := make([]string, 0)
	for _, ps := range paths {
		p, _ := path.Parse(ps)
		if p.Match(selector) {
			matchedPaths = append(matchedPaths, ps)
		}
	}
	b, err := json.Marshal(matchedPaths)
	if err != nil {
		log.Error().Err(err).Msg("Marshal response")
		w.WriteHeader(500)
		return
	}
	if _, err := write(b); err != nil {
		w.WriteHeader(500)
		return
	}
}
