package daemonapi

import (
	"net/http"

	"github.com/goccy/go-json"
	"github.com/rs/zerolog"
)

func (a *DaemonApi) PostDaemonLogsControl(w http.ResponseWriter, r *http.Request) {
	var (
		payload PostDaemonLogsControl
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
	response := ResponseText("new log level " + payload.Level)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
