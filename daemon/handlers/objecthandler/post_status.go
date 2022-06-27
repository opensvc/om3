package objecthandler

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
)

type (
	PostObjectStatus struct {
		Path string          `json:"path"`
		Data instance.Status `json:"data"`
	}

	response struct {
		status int    `json:"status"`
		info   string `json:"info"`
	}
)

func PostStatus(w http.ResponseWriter, r *http.Request) {
	write, log := handlerhelper.GetWriteAndLog(w, r, "objecthandler.PostStatus")
	log.Debug().Msg("starting")
	postStatus := PostObjectStatus{}
	if reqBody, err := ioutil.ReadAll(r.Body); err != nil {
		log.Error().Err(err).Msg("read body request")
		w.WriteHeader(500)
		return
	} else if err := json.Unmarshal(reqBody, &postStatus); err != nil {
		log.Error().Err(err).Msg("request body unmarshal")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if p, err := path.Parse(postStatus.Path); err != nil {
		log.Error().Err(err).Msg("path.Parse")
		w.WriteHeader(http.StatusBadRequest)
		return
	} else {
		dataCmd := daemonctx.DaemonDataCmd(r.Context())
		log.Debug().Msgf("SetInstanceStatus on %s", postStatus.Path)
		if err := daemondata.SetInstanceStatus(dataCmd, p, postStatus.Data); err != nil {
			log.Error().Err(err).Msgf("SetInstanceStatus %s", p)
		}
	}

	response := response{0, "instance status pushed pending ops"}
	b, err := json.Marshal(response)
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
