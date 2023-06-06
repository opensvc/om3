package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/handlers/dispatchhandler"
)

func (a *DaemonApi) GetDaemonRunning(ctx echo.Context) error {
	// TODO verify if drop support of dispatchhandler
	dispatchhandler.New(a.getDaemonRunning, http.StatusOK, 1)(ctx.Response(), ctx.Request())
	return nil
}

func (a *DaemonApi) getDaemonRunning(w http.ResponseWriter, r *http.Request) {
	running := a.Daemon.Running()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(running)
	w.WriteHeader(http.StatusOK)
}
