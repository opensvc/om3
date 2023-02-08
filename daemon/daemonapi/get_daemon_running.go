package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/handlers/dispatchhandler"
)

func (a *DaemonApi) GetDaemonRunning(w http.ResponseWriter, r *http.Request) {
	// TODO verify if drop support of dispatchhandler
	dispatchhandler.New(a.getDaemonRunning, http.StatusOK, 1)(w, r)
}

func (a *DaemonApi) getDaemonRunning(w http.ResponseWriter, r *http.Request) {
	daemon := daemonctx.Daemon(r.Context())
	running := daemon.Running()
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(running)
}
