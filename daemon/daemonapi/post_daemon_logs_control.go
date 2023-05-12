package daemonapi

import (
	"net/http"

	"github.com/goccy/go-json"
	"github.com/opensvc/om3/daemon/api"
	"github.com/rs/zerolog"
)

func (a *DaemonApi) PostDaemonLogsControl(w http.ResponseWriter, r *http.Request) {
	var (
		payload api.PostDaemonLogsControl
	)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		WriteProblemf(w, http.StatusBadRequest, "Invalid body", "%s", err)
		return
	}
	var level string
	if payload.Level != "none" {
		level = string(payload.Level)
	}
	newLevel, err := zerolog.ParseLevel(string(level))
	if err != nil {
		WriteProblemf(w, http.StatusBadRequest, "Invalid body", "Error parsing 'level': %s", err)
		return
	}
	zerolog.SetGlobalLevel(newLevel)
	WriteProblemf(w, http.StatusOK, "New log level", "%s", payload.Level)
}
