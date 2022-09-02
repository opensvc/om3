package objecthandler

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/iancoleman/orderedmap"

	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
	"opensvc.com/opensvc/util/file"
)

type (
	GetObjectConfigOptions struct {
		Path        string `json:"path"`
		Evaluate    bool   `json:"evaluate"`
		Impersonate string `json:"impersonate"`
	}
	GetObjectConfig struct {
		Options GetObjectConfigOptions `json:"options"`
	}

	Data struct {
		Updated time.Time              `json:"mtime"`
		Data    *orderedmap.OrderedMap `json:"data"`
	}
	GetConfigResponse struct {
		Status int  `json:"status"`
		Data   Data `json:"data"`
	}
)

func GetConfig(w http.ResponseWriter, r *http.Request) {
	var b []byte
	var err error
	var resp = GetConfigResponse{
		Data: Data{},
	}
	var data *orderedmap.OrderedMap
	write, log := handlerhelper.GetWriteAndLog(w, r, "objecthandler.GetConfig")
	log.Debug().Msg("starting")
	payload := GetObjectConfigOptions{}
	if r.Body == nil {
		log.Info().Msg("can't read request body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if reqBody, err := io.ReadAll(r.Body); err != nil {
		log.Info().Err(err).Msg("read body request")
		w.WriteHeader(http.StatusBadRequest)
		return
	} else {
		if err := json.Unmarshal(reqBody, &payload); err != nil {
			log.Info().Err(err).Msg("request body unmarshal")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	objPath, err := path.Parse(payload.Path)
	if err != nil {
		log.Info().Err(err).Msgf("invalid path: %s", payload.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if payload.Impersonate != "" && !payload.Evaluate {
		// Force evaluate when impersonate
		payload.Evaluate = true
	}
	filename := objPath.ConfigFile()
	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		log.Error().Msgf("configFile no present(mtime) %s %s (may be deleted)", filename, mtime)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	resp.Data.Updated = mtime
	data, err = configData(objPath, payload.Evaluate, payload.Impersonate)
	if err != nil {
		log.Error().Err(err).Msgf("can't get configData for %s %s", objPath, filename)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if file.ModTime(filename) != mtime {
		log.Error().Msgf("file has changed %s", filename)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	data.Set("metadata", objPath.ToMetadata())
	resp.Data.Data = data
	b, err = json.Marshal(resp)
	if err != nil {
		log.Error().Err(err).Msgf("marshal response error %s %s", objPath, filename)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err := write(b); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func configData(p path.T, eval bool, impersonate string) (data *orderedmap.OrderedMap, err error) {
	var o object.Configurer
	var config rawconfig.T
	if o, err = object.NewConfigurer(p, object.WithVolatile(true)); err != nil {
		return
	}
	if eval {
		if impersonate != "" {
			config, err = o.EvalConfigAs(impersonate)
		} else {
			config, err = o.EvalConfig()
		}
	} else {
		config, err = o.PrintConfig()
	}
	if err != nil {
		return
	}
	return config.Data, nil
}
