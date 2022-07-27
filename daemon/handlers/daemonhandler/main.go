/*
	Package daemonhandler manage daemon handlers for listeners
*/
package daemonhandler

import (
	"encoding/json"
	"net/http"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/handlers/dispatchhandler"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
	"opensvc.com/opensvc/util/pubsub"
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

func Events(w http.ResponseWriter, r *http.Request) {
	write, logger := handlerhelper.GetWriteAndLog(w, r, "daemonhandler.Events")
	logger.Debug().Msg("starting")
	ctx := r.Context()
	bus := pubsub.BusFromContext(ctx)
	done := make(chan bool)
	var httpBody bool
	if r.Header.Get("accept") == "text/event-stream" {
		httpBody = true
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-control", "no-store")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Transfer-Encoding", "chunked")
	}

	w.WriteHeader(http.StatusOK)
	evChan := make(chan event.Event)
	getEvent := func(ev event.Event) {
		evChan <- ev
	}
	writeEvent := func(ev event.Event) {
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
		select {
		case <-ctx.Done():
			return
		default:
		}
		if _, err := write(msg); err != nil {
			logger.Error().Err(err).Msg("write failure")
			done <- true
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	subId := daemonps.SubEvent(bus, "lsnr-handler-event "+daemonctx.Uuid(r.Context()).String(), getEvent)
	defer daemonps.UnSubEvent(bus, subId)
	go func() {
		for {
			select {
			case <-ctx.Done():
				done <- true
				return
			case ev := <-evChan:
				writeEvent(ev)
			}
		}
	}()

	<-done
	logger.Debug().Msg("done")
}
