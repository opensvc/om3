/*
	Package daemonhandler manage daemon handlers for listeners
*/
package daemonhandler

import (
	"net/http"

	"opensvc.com/opensvc/daemon/listener/mux/muxctx"
)

func Running(w http.ResponseWriter, r *http.Request) {
	funcName := "daemonhandler.Running"
	daemon := muxctx.Daemon(r.Context())
	if daemon.Running() {
		write(w, r, funcName, "running")
	} else {
		write(w, r, funcName, "not running")
	}
}

func Stop(w http.ResponseWriter, r *http.Request) {
	funcName := "daemonhandler.Stop"
	logger := muxctx.Logger(r.Context())
	logger.Info().Msg(funcName + "...")
	daemon := muxctx.Daemon(r.Context())
	if daemon.Running() {
		msg := funcName + ": stopping"
		logger.Info().Msg(msg)
		if err := daemon.StopAndQuit(); err != nil {
			msg := funcName + ": StopAndQuit error"
			logger.Error().Err(err).Msg(msg)
			write(w, r, funcName, msg+" "+err.Error())
		}
	} else {
		msg := funcName + ": no daemon to stop"
		logger.Info().Msg(msg)
		write(w, r, funcName, msg)
	}
}

func write(w http.ResponseWriter, r *http.Request, funcName, msg string) {
	_, err := w.Write([]byte(msg))
	if err != nil {
		logger := muxctx.Logger(r.Context())
		logger.Debug().Err(err).Msg(funcName + " write error")
	}
}
