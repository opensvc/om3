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
	logger := muxctx.Logger(r.Context()).With().Str("func", funcName).Logger()
	daemon := muxctx.Daemon(r.Context())
	logger.Debug().Msg("starting")
	if daemon.Running() {
		logger.Info().Msg("daemon is running")
		_, _ = write(w, r, funcName, []byte("running"))
	} else {
		logger.Info().Msg("daemon is stopped")
		_, _ = write(w, r, funcName, []byte("not running"))
	}
}

func Stop(w http.ResponseWriter, r *http.Request) {
	funcName := "daemonhandler.Stop"
	logger := muxctx.Logger(r.Context()).With().Str("func", funcName).Logger()
	logger.Debug().Msg("starting")
	daemon := muxctx.Daemon(r.Context())
	if daemon.Running() {
		msg := funcName + ": stopping"
		logger.Info().Msg(msg)
		if err := daemon.StopAndQuit(); err != nil {
			msg := funcName + ": StopAndQuit error"
			logger.Error().Err(err).Msg(msg)
			_, _ = write(w, r, funcName, []byte(msg+" "+err.Error()))
		}
	} else {
		msg := funcName + ": no daemon to stop"
		logger.Info().Msg(msg)
		_, _ = write(w, r, funcName, []byte(msg))
	}
}

func write(w http.ResponseWriter, r *http.Request, funcName string, b []byte) (int, error) {
	written, err := w.Write(b)
	if err != nil {
		logger := muxctx.Logger(r.Context())
		logger.Debug().Err(err).Msg(funcName + " write error")
		return written, err
	}
	return written, nil
}
