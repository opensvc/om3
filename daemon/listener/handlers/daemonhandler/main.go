/*
	Package daemonhandler manage daemon handlers for listeners
*/
package daemonhandler

import (
	"encoding/json"
	"net/http"
	"time"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/listener/handlers/dispatchhandler"
	"opensvc.com/opensvc/util/timestamp"
)

var (
	Running = dispatchhandler.New(running, http.StatusOK, 1)
)

func running(w http.ResponseWriter, r *http.Request) {
	funcName := "daemonhandler.Running"
	logger := daemonctx.Logger(r.Context()).With().Str("func", funcName).Logger()
	daemon := daemonctx.Daemon(r.Context())
	logger.Debug().Msg("starting")
	response := daemon.Running()
	b, err := json.Marshal(response)
	if err != nil {
		logger.Error().Err(err).Msg("Marshal response")
		return
	}
	_, _ = write(w, r, funcName, b)
}

func Stop(w http.ResponseWriter, r *http.Request) {
	funcName := "daemonhandler.Stop"
	logger := daemonctx.Logger(r.Context()).With().Str("func", funcName).Logger()
	logger.Debug().Msg("starting")
	daemon := daemonctx.Daemon(r.Context())
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

func Events(w http.ResponseWriter, r *http.Request) {
	funcName := "daemonhandler.Events"
	logger := daemonctx.Logger(r.Context()).With().Str("func", funcName).Logger()
	logger.Debug().Msg("starting")
	var httpBody bool
	if r.Header.Get("accept") == "text/event-stream" {
		httpBody = true
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-control", "no-store")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Transfer-Encoding", "chunked")
	}

	w.WriteHeader(http.StatusOK)

	for i := 0; i < 3; i++ {
		rawMsg := json.RawMessage("\"demo msg xxx\"")
		ev := event.Event{
			Kind:      "demo",
			ID:        uint64(i),
			Timestamp: timestamp.Now(),
			Data:      &rawMsg,
		}
		b, err := json.Marshal(ev)
		if err != nil {
			logger.Error().Err(err).Interface("event", ev).Msg("Marshal")
			return
		}
		logger.Debug().Msgf("send fragment: %#v", ev)

		var endMsg, msg []byte
		if httpBody {
			endMsg = []byte("\n\n")
			msg = append([]byte("data: "), b...)
		} else {
			endMsg = []byte("\n\n\x00")
			msg = append([]byte(""), b...)
		}

		msg = append(msg, endMsg...)
		if _, err := write(w, r, funcName, msg); err != nil {
			logger.Error().Err(err).Msg("write failure")
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(1000 * time.Millisecond)
	}
	logger.Debug().Msg("done")
}

func write(w http.ResponseWriter, r *http.Request, funcName string, b []byte) (int, error) {
	written, err := w.Write(b)
	if err != nil {
		logger := daemonctx.Logger(r.Context())
		logger.Debug().Err(err).Msg(funcName + " write error")
		return written, err
	}
	return written, nil
}
