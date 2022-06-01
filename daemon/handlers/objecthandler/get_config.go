package objecthandler

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	GetObjectConfigOptions struct {
		Path string `json:"path"`
	}
	GetObjectConfig struct {
		Options GetObjectConfigOptions `json:"options"`
	}

	Data struct {
		Updated timestamp.T `json:"mtime"`
		Data    string      `json:"data"`
	}
	GetConfigResponse struct {
		Status int  `json:"status"`
		Data   Data `json:"data"`
	}
)

func GetConfig(w http.ResponseWriter, r *http.Request) {
	write, log := handlerhelper.GetWriteAndLog(w, r, "objecthandler.GetConfig")
	log.Debug().Msg("starting")
	payload := GetObjectConfig{}
	if r.Body == nil {
		log.Error().Msg("can't read request body")
		w.WriteHeader(500)
		return
	}
	if reqBody, err := ioutil.ReadAll(r.Body); err != nil {
		log.Error().Err(err).Msg("read body request")
		w.WriteHeader(500)
		return
	} else {
		if err := json.Unmarshal(reqBody, &payload); err != nil {
			log.Error().Err(err).Msg("request body unmarshal")
			w.WriteHeader(500)
			return
		}
	}

	pathEtc := rawconfig.Node.Paths.Etc
	filename := pathEtc + "/" + payload.Options.Path + ".conf"
	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		log.Error().Msgf("configFile no present(mtime) %s %s", filename, mtime)
		w.WriteHeader(500)
		return
	}
	readed, err := file.ReadAll(filename)
	if err != nil {
		log.Error().Msgf("can't read %s", filename)
		w.WriteHeader(500)
		return
	}
	if file.ModTime(filename) != mtime {
		log.Error().Msgf("file has changed %s", filename)
		w.WriteHeader(500)
		return
	}
	resp := GetConfigResponse{
		Status: 0,
		Data: Data{
			Updated: timestamp.New(mtime),
			Data:    string(readed),
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		log.Error().Err(err).Msgf("marshal response error %s", filename)
		w.WriteHeader(500)
		return
	}
	if _, err := write(b); err != nil {
		w.WriteHeader(500)
		return
	}
}
