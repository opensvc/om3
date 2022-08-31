package objecthandler

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
	"opensvc.com/opensvc/util/file"
)

type (
	GetObjectConfigOptions struct {
		Path string `json:"path"`
	}
	GetObjectConfig struct {
		Options GetObjectConfigOptions `json:"options"`
	}

	Data struct {
		Updated time.Time `json:"mtime"`
		Data    string    `json:"data"`
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
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if reqBody, err := ioutil.ReadAll(r.Body); err != nil {
		log.Error().Err(err).Msg("read body request")
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else {
		if err := json.Unmarshal(reqBody, &payload); err != nil {
			log.Error().Err(err).Msg("request body unmarshal")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	pathEtc := rawconfig.Paths.Etc
	objPath, err := path.Parse(payload.Options.Path)
	if err != nil {
		log.Error().Err(err).Msgf("invalid path: %s", payload.Options.Path)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var prefix string
	if objPath.Namespace != "root" {
		prefix = "namespaces/"
	}
	filename := pathEtc + "/" + prefix + objPath.String() + ".conf"
	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		log.Error().Msgf("configFile no present(mtime) %s %s", filename, mtime)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	b, err := os.ReadFile(filename)
	if err != nil {
		log.Error().Msgf("can't read %s", filename)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if file.ModTime(filename) != mtime {
		log.Error().Msgf("file has changed %s", filename)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	resp := GetConfigResponse{
		Status: 0,
		Data: Data{
			Updated: mtime,
			Data:    string(b),
		},
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		log.Error().Err(err).Msgf("marshal response error %s", filename)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err := write(respB); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
