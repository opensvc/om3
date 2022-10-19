/*
Package daemonhandler manage daemon handlers for listeners
*/
package daemonhandler

import (
	"encoding/json"
	"net/http"

	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/handlers/dispatchhandler"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
)

var (
	Running = dispatchhandler.New(running, http.StatusOK, 1)
)

func running(w http.ResponseWriter, r *http.Request) {
	write, logger := handlerhelper.GetWriteAndLog(w, r, "daemonhandler.Running")
	daemon := daemonctx.Daemon(r.Context())
	logger.Debug().Msg("starting")
	response := daemon.Running()
	b, err := json.Marshal(response)
	if err != nil {
		logger.Error().Err(err).Msg("Marshal response")
		return
	}
	_, _ = write(b)
}
