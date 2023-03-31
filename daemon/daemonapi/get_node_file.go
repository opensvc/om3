package daemonapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/opensvc/om3/daemon/handlers/handlerhelper"
	"github.com/opensvc/om3/util/file"
)

func (a *DaemonApi) GetNodeFile(w http.ResponseWriter, r *http.Request, params GetNodeFileParams) {
	var (
		b        []byte
		err      error
		filename string
	)
	write, log := handlerhelper.GetWriteAndLog(w, r, "nodehandler.GetNodeFile")
	log.Debug().Msg("starting")

	if params.Name == "" {
		log.Warn().Err(err).Msgf("invalid file name: %s", params.Name)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch params.Kind {
	case "drbd":
		filename = fmt.Sprintf("/etc/drbd.d/%s.res", params.Name)
	default:
		log.Warn().Err(err).Msgf("invalid file kind: %s", params.Kind)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		log.Info().Msgf("file %s not found", filename)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	resp := ObjectFile{
		Mtime: mtime,
	}
	resp.Data, err = os.ReadFile(filename)

	if err != nil {
		log.Info().Err(err).Msgf("Readfile %s (may be deleted)", filename)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if file.ModTime(filename) != resp.Mtime {
		log.Info().Msgf("file has changed %s", filename)
		w.WriteHeader(http.StatusTooEarly)
		return
	}

	b, err = json.Marshal(resp)
	if err != nil {
		log.Error().Err(err).Msgf("marshal response error %s", filename)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err := write(b); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
