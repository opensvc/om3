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

func Stop(w http.ResponseWriter, r *http.Request) {
	write, logger := handlerhelper.GetWriteAndLog(w, r, "daemonhandler.Stop")
	logger.Debug().Msg("starting")
	daemon := daemonctx.Daemon(r.Context())
	if daemon.Running() {
		logger.Info().Msg("stopping")
		if err := daemon.Stop(); err != nil {
			msg := "Stop"
			logger.Error().Err(err).Msg(msg)
			_, _ = write([]byte(msg + " " + err.Error()))
		}
	} else {
		msg := "no daemon to stop"
		logger.Info().Msg(msg)
		_, _ = write([]byte(msg))
	}
}
