package daemonapi

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/handlers/handlerhelper"
	"github.com/opensvc/om3/util/file"
)

func (a *DaemonApi) GetObjectFile(w http.ResponseWriter, r *http.Request, params api.GetObjectFileParams) {
	_, log := handlerhelper.GetWriteAndLog(w, r, "objecthandler.GetObjectFile")
	log.Debug().Msg("starting")

	objPath, err := path.Parse(params.Path)
	if err != nil {
		log.Warn().Err(err).Msgf("invalid path: %s", params.Path)
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
	resp := api.ObjectFile{
		Mtime: mtime,
	}
	resp.Data, err = os.ReadFile(filename)

	if err != nil {
		log.Info().Err(err).Msgf("readfile %s %s (may be deleted)", objPath, filename)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if file.ModTime(filename) != resp.Mtime {
		log.Info().Msgf("file has changed %s", filename)
		w.WriteHeader(http.StatusTooEarly)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error().Err(err).Msgf("marshal response error %s %s", objPath, filename)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
