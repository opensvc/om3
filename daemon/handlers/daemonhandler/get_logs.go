package daemonhandler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/slog"
	"opensvc.com/opensvc/daemon/daemonauth"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
)

type logPayload struct {
	Filters map[string]interface{}
}

func getLogPayload(w http.ResponseWriter, r *http.Request) (logPayload, error) {
	var payload logPayload

	if reqBody, err := io.ReadAll(r.Body); err != nil {
		return payload, errors.Wrap(err, "read body request")
	} else if len(reqBody) == 0 {
		// pass
	} else if err := json.Unmarshal(reqBody, &payload); err != nil {
		return payload, errors.Wrap(err, "request body unmarshal")
	}
	return payload, nil
}

// GetNodeBacklog return the node log
func GetNodeBacklog(w http.ResponseWriter, r *http.Request) {
	grants := daemonauth.UserGrants(r)
	if !grants.HasRoot() {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	_, logger := handlerhelper.GetWriteAndLog(w, r, "daemonhandler.Backlogs")
	logger.Debug().Msg("starting")

	// parse request body for parameters
	payload, err := getLogPayload(w, r)
	if err != nil {
		log.Error().Err(err).Msg("parse request body")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	events, err := slog.GetEventsFromNode(payload.Filters)
	if err != nil {
		log.Error().Err(err).Msg("read node events")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(events); err != nil {
		log.Error().Err(err).Msg("encode node events")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// GetNodeLog feeds node log in sse format.
func GetNodeLog(w http.ResponseWriter, r *http.Request) {
	grants := daemonauth.UserGrants(r)
	if !grants.HasRoot() {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	write, logger := handlerhelper.GetWriteAndLog(w, r, "daemonhandler.Logs")
	logger.Debug().Msg("starting")

	// parse request body for parameters
	payload, err := getLogPayload(w, r)
	if err != nil {
		log.Error().Err(err).Msg("parse request body")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// prepare the SSE response
	ctx := r.Context()
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

	writeEvent := func(ev slog.Event) {
		/*
			if !allowEvent(r, ev, payload) {
				logger.Debug().Interface("event", ev).Msg("hide denied event")
				return
			}
		*/
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
	stream, err := slog.GetEventStreamFromNode(payload.Filters)
	if err != nil {
		log.Error().Err(err).Msg("get node stream")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	go func() {
		for ev := range stream.Events() {
			writeEvent(ev)
		}
	}()

	<-done
	logger.Debug().Msg("done")
}
