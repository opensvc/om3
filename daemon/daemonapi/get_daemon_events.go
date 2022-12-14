package daemonapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/event/sseevent"
	"opensvc.com/opensvc/daemon/daemonauth"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/pubsub"
)

// GetDaemonEvents feeds publications in rss format.
// TODO: Honor subscribers params.
func (a *DaemonApi) GetDaemonEvents(w http.ResponseWriter, r *http.Request, params GetDaemonEventsParams) {
	var (
		handlerName = "GetDaemonEvents"
		limit       uint64
		maxDuration = 5 * time.Second
		eventCount  uint64
		payload     GetDaemonEventsJSONBody
	)
	log := getLogger(r, handlerName)
	log.Debug().Msg("starting")
	defer log.Debug().Msg("done")

	if params.Limit != nil {
		limit = uint64(*params.Limit)
	}
	if params.Duration != nil {
		if v, err := converters.Duration.Convert(*params.Duration); err != nil {
			log.Info().Err(err).Msgf("invalid duration: %s", *params.Duration)
			sendError(w, http.StatusBadRequest, "invalid duration")
		} else {
			maxDuration = *v.(*time.Duration)
		}
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Warn().Err(err).Msgf("decode body")
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	grants := daemonauth.UserGrants(r)
	if !grants.HasRoot() {
		log.Info().Msg("not allowed, need grant root")
		w.WriteHeader(http.StatusForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), maxDuration)
	defer cancel()
	bus := pubsub.BusFromContext(ctx)
	if r.Header.Get("accept") == "text/event-stream" {
		setStreamHeaders(w)
	}

	filterArgs, err := payload.filterArgs()
	if err != nil {
		log.Warn().Err(err).Msgf("invalid filter")
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(filterArgs) == 0 {
		filterArgs = []AddFilterArgs{
			{Kind: msgbus.DataUpdated{}},
		}
	}

	name := fmt.Sprintf("lsnr-handler-event %s from %s %s", handlerName, r.RemoteAddr, daemonctx.Uuid(r.Context()))
	AnnounceSub(bus, name)
	defer AnnounceUnSub(bus, name)

	sub := bus.SubWithTimeout(name, time.Second)

	for _, filter := range filterArgs {
		sub.AddFilter(filter.Kind, filter.Labels...)
	}
	//sub.AddFilter(msgbus.DataUpdated{})
	//sub.AddFilter(nil, labels...)
	sub.Start()
	defer sub.Stop()

	w.WriteHeader(http.StatusOK)

	if f, ok := w.(http.Flusher); ok {
		// don't wait first event to flush response
		f.Flush()
	}
	eventC := event.ChanFromAny(ctx, sub.C)
	sseWriter := sseevent.NewWriter(w)
	for ev := range eventC {
		log.Debug().Msgf("write event %s", ev.Kind)
		if _, err := sseWriter.Write(ev); err != nil {
			log.Debug().Err(err).Msgf("write event %s", ev.Kind)
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		eventCount++
		if limit > 0 && eventCount >= limit {
			return
		}
	}
}

//func allowPatchEvent(r *http.Request, ev event.Event, selected path.M) bool {
//	log := daemonlogctx.Logger(r.Context()).With().Str("func", "daemonhandler.allowPatchEvent").Logger()
//	log.Warn().Msg("TODO")
//	return true
//}
//
//func allowEventEvent(r *http.Request, ev event.Event, selected path.M) bool {
//	log := daemonlogctx.Logger(r.Context()).With().Str("func", "daemonhandler.allowEventEvent").Logger()
//	var d struct {
//		Path path.T `json:"path"`
//	}
//	if err := json.Unmarshal([]byte(ev.Data), &d); err != nil {
//		log.Error().Err(err).Msg("extract object path from event event")
//		return false
//	}
//	if _, ok := selected[d.Path.String()]; ok {
//		return true
//	}
//	return false
//}
//
//func allowEvent(r *http.Request, ev event.Event, payload eventsPayload) bool {
//	log := daemonlogctx.Logger(r.Context()).With().Str("func", "daemonhandler.allowEvent").Logger()
//	grants := daemonauth.UserGrants(r)
//	if grants.HasRoot() {
//		return true
//	}
//
//	// selected paths
//	paths, err := objectselector.NewSelection(
//		payload.Selector,
//		objectselector.SelectionWithLocal(true),
//	).Expand()
//	if err != nil {
//		log.Error().Err(err).Msg("expand selector")
//		return false
//	}
//	grants.FilterPaths(r, daemonauth.RoleGuest, paths)
//	selected := paths.StrMap()
//
//	switch {
//	case ev.Kind == "patch":
//		return allowPatchEvent(r, ev, selected)
//	case ev.Kind == "event":
//		return allowPatchEvent(r, ev, selected)
//	case ev.Kind == "full":
//		// TODO: does that still exist in b3 ?
//		return true
//	default:
//		return false
//	}
//}

type (
	AddFilterArgs struct {
		Kind   any
		Labels []pubsub.Label
	}
)

var (
	invalidKind = errors.New("invalid kind")
)

func (b *GetDaemonEventsJSONBody) filterArgs() (filters []AddFilterArgs, err error) {
	if b.Filter == nil {
		return
	}
	for _, filter := range *b.Filter {
		filterEntry := AddFilterArgs{}
		if filter.Kind == nil {
			continue
		}
		filterEntry.Kind, err = b.kindToT(*filter.Kind)
		if err != nil {
			continue
		}

		if filter.Labels != nil {
			for _, l := range *filter.Labels {
				if l.Name != nil && l.Value != nil {
					filterEntry.Labels = append(filterEntry.Labels, pubsub.Label{*l.Name, *l.Value})
				}
			}
		}
	}
	return
}

func (b *GetDaemonEventsJSONBody) kindToT(kind string) (any, error) {
	var i interface{}
	switch kind {
	case "DataUpdated":
		i = msgbus.DataUpdated{}
	case "HbPing":
		i = msgbus.HbPing{}
	case "HbStale":
		i = msgbus.HbStale{}
	default:
		return nil, invalidKind
	}
	return i, nil
}
