package daemonhandler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/objectselector"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/daemonauth"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/handlers/handlerhelper"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/pubsub"
)

type eventsPayload struct {
	Namespace string
	Selector  string
	Relatives bool
}

func allowPatchEvent(r *http.Request, ev event.Event, selected path.M) bool {
	log := daemonlogctx.Logger(r.Context()).With().Str("func", "daemonhandler.allowPatchEvent").Logger()
	log.Warn().Msg("TODO")
	return true
}

func allowEventEvent(r *http.Request, ev event.Event, selected path.M) bool {
	log := daemonlogctx.Logger(r.Context()).With().Str("func", "daemonhandler.allowEventEvent").Logger()
	var d struct {
		Path path.T `json:"path"`
	}
	if err := json.Unmarshal([]byte(ev.Data), &d); err != nil {
		log.Error().Err(err).Msg("extract object path from event event")
		return false
	}
	if _, ok := selected[d.Path.String()]; ok {
		return true
	}
	return false
}

func allowEvent(r *http.Request, ev event.Event, payload eventsPayload) bool {
	log := daemonlogctx.Logger(r.Context()).With().Str("func", "daemonhandler.allowEvent").Logger()
	grants := daemonauth.UserGrants(r)
	if grants.HasRoot() {
		return true
	}

	// selected paths
	paths, err := objectselector.NewSelection(
		payload.Selector,
		objectselector.SelectionWithLocal(true),
	).Expand()
	if err != nil {
		log.Error().Err(err).Msg("expand selector")
		return false
	}
	grants.FilterPaths(r, daemonauth.RoleGuest, paths)
	selected := paths.StrMap()

	switch {
	case ev.Kind == "patch":
		return allowPatchEvent(r, ev, selected)
	case ev.Kind == "event":
		return allowPatchEvent(r, ev, selected)
	case ev.Kind == "full":
		// TODO: does that still exist in b3 ?
		return true
	default:
		return false
	}
}

func getEventsPayload(w http.ResponseWriter, r *http.Request) (eventsPayload, error) {
	var payload eventsPayload

	if reqBody, err := io.ReadAll(r.Body); err != nil {
		return payload, errors.Wrap(err, "read body request")
	} else if len(reqBody) == 0 {
		// pass
	} else if err := json.Unmarshal(reqBody, &payload); err != nil {
		return payload, errors.Wrap(err, "request body unmarshal")
	}

	// default to open selection
	if payload.Selector == "" {
		payload.Selector = "**"
	}
	if payload.Namespace != "" {
		payload.Selector += fmt.Sprintf("+*/%s/*", payload.Namespace)
	}
	return payload, nil
}

// Events feeds Patch or Events in rss format.
// TODO: Honor namespace and selection parameters.
func Events(w http.ResponseWriter, r *http.Request) {
	write, logger := handlerhelper.GetWriteAndLog(w, r, "daemonhandler.Events")
	logger.Debug().Msg("starting")

	// parse request body for parameters
	payload, err := getEventsPayload(w, r)
	if err != nil {
		log.Error().Err(err).Msg("parse request body")
		w.WriteHeader(http.StatusInternalServerError)
	}

	// prepare the SSE response
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

	// start go routines to write events as they come
	evChan := make(chan event.Event)
	getEvent := func(ev event.Event) {
		evChan <- ev
	}
	writeEvent := func(ev event.Event) {
		if !allowEvent(r, ev, payload) {
			logger.Debug().Interface("event", ev).Msg("hide denied event")
			return
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
	subId := msgbus.SubEventWithTimeout(bus, "lsnr-handler-event "+daemonctx.Uuid(r.Context()).String(), getEvent, time.Second)
	defer msgbus.UnSubEvent(bus, subId)
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
