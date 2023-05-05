package daemonapi

import (
	"net/http"

	"github.com/goccy/go-json"
	"github.com/opensvc/om3/daemon/api"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func (a *DaemonApi) PostDaemonLogsControl(w http.ResponseWriter, r *http.Request) {
	var (
		payload api.PostDaemonLogsControl
	)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	var level string
	if payload.Level != "none" {
		level = string(payload.Level)
	}
	newLevel, err := zerolog.ParseLevel(string(level))
	if err != nil {
		sendErrorf(w, http.StatusBadRequest, "invalid level %s", payload.Level)
		return
	}
	zerolog.SetGlobalLevel(newLevel)
	response := api.ResponseText("new log level " + payload.Level)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("json encode")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
