package objecthandler

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
	"opensvc.com/opensvc/util/file"
)

type (
	GetObjectConfigFileOptions struct {
		Path string `json:"path"`
	}

	GetObjectConfigFile struct {
		Options GetObjectConfigFileOptions `json:"options"`
	}

	GetObjectConfigFileResponseData struct {
		Updated time.Time `json:"mtime"`
		Data    []byte    `json:"data"`
	}
	GetObjectConfigFileResponse struct {
		Status int                             `json:"status"`
		Data   GetObjectConfigFileResponseData `json:"data"`
	}
)

// GetConfigFile
// {"status": 0, "data": buff, "mtime": mtime}
func GetConfigFile(w http.ResponseWriter, r *http.Request) {
	var b []byte
	var err error
	resp := GetObjectConfigFileResponse{
		Data: GetObjectConfigFileResponseData{},
	}
	write, log := handlerhelper.GetWriteAndLog(w, r, "objecthandler.GetConfigFile")
	log.Debug().Msg("starting")
	payload := GetObjectConfigOptions{}
	if r.Body == nil {
		log.Warn().Msg("can't read request body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if reqBody, err := io.ReadAll(r.Body); err != nil {
		log.Warn().Err(err).Msg("read body request")
		w.WriteHeader(http.StatusBadRequest)
		return
	} else {
		if err := json.Unmarshal(reqBody, &payload); err != nil {
			log.Warn().Err(err).Msg("request body unmarshal")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	objPath, err := path.Parse(payload.Path)
	if err != nil {
		log.Warn().Err(err).Msgf("invalid path: %s", payload.Path)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	filename := objPath.ConfigFile()

	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		log.Info().Msgf("configFile no present(mtime) %s %s", filename, mtime)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	resp.Data.Updated = mtime
	resp.Data.Data, err = os.ReadFile(filename)

	if err != nil {
		log.Info().Err(err).Msgf("readfile %s %s (may be deleted)", objPath, filename)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if file.ModTime(filename) != resp.Data.Updated {
		log.Info().Msgf("file has changed %s", filename)
		w.WriteHeader(http.StatusTooEarly)
		return
	}

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
