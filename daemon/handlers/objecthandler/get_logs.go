package objecthandler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/slog"
	"github.com/opensvc/om3/daemon/daemonauth"
	"github.com/opensvc/om3/daemon/handlers/handlerhelper"
)

type logPayload struct {
	Paths   path.L
	Filters map[string]interface{}
}

func getLogPayload(w http.ResponseWriter, r *http.Request) (logPayload, error) {
	var payload logPayload

	if reqBody, err := io.ReadAll(r.Body); err != nil {
		return payload, fmt.Errorf("read body request: %w", err)
	} else if len(reqBody) == 0 {
		// pass
	} else if err := json.Unmarshal(reqBody, &payload); err != nil {
		return payload, fmt.Errorf("request body unmarshal: %w", err)
	}
	return payload, nil
}

// GetObjectsBacklog return the object log
func GetObjectsBacklog(w http.ResponseWriter, r *http.Request) {
	_, logger := handlerhelper.GetWriteAndLog(w, r, "objecthandler.Backlogs")
	logger.Debug().Msg("starting")

	// parse request body for parameters
	payload, err := getLogPayload(w, r)
	if err != nil {
		log.Error().Err(err).Msg("parse request body")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Debug().Interface("payload", payload).Msg("")
	grants := daemonauth.UserGrants(r)
	allowed := make(path.L, 0)
	for _, p := range payload.Paths {
		if grants.HasRoot() || grants.MatchPath(r, daemonauth.RoleGuest, p) {
			allowed = append(allowed, p)
		}
	}
	log.Debug().Interface("allowed", allowed).Msg("")

	events, err := slog.GetEventsFromObjects(allowed, payload.Filters)
	if err != nil {
		log.Error().Err(err).Msg("read object events")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(events); err != nil {
		log.Error().Err(err).Msg("encode object events")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// GetObjectsLog feeds object log in sse format.
func GetObjectsLog(w http.ResponseWriter, r *http.Request) {
	write, logger := handlerhelper.GetWriteAndLog(w, r, "objecthandler.Logs")
	logger.Debug().Msg("starting")

	// parse request body for parameters
	payload, err := getLogPayload(w, r)
	if err != nil {
		log.Error().Err(err).Msg("parse request body")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	grants := daemonauth.UserGrants(r)
	allowed := make(path.L, 0)
	for _, p := range payload.Paths {
		if grants.HasRoot() || grants.MatchPath(r, daemonauth.RoleGuest, p) {
			allowed = append(allowed, p)
		}
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
	stream, err := slog.GetEventStreamFromObjects(allowed, payload.Filters)
	if err != nil {
		log.Error().Err(err).Msg("get object stream")
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
