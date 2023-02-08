package daemonapi

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/event/sseevent"
	"github.com/opensvc/om3/daemon/daemonauth"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/converters"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	Filter struct {
		Kind   any
		Labels []pubsub.Label
	}
)

// GetDaemonEvents feeds publications in rss format.
// TODO: Honor subscribers params.
func (a *DaemonApi) GetDaemonEvents(w http.ResponseWriter, r *http.Request, params GetDaemonEventsParams) {
	var (
		handlerName = "GetDaemonEvents"
		limit       uint64
		eventCount  uint64

		ctx    context.Context = r.Context()
		cancel context.CancelFunc
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
			ctx, cancel = context.WithTimeout(ctx, *v.(*time.Duration))
			defer cancel()
		}
	}

	grants := daemonauth.UserGrants(r)
	if !grants.HasAnyRole(daemonauth.RoleRoot, daemonauth.RoleJoin) {
		log.Info().Msg("not allowed, need at least 'root' or 'join' grant")
		w.WriteHeader(http.StatusForbidden)
		return
	}

	bus := pubsub.BusFromContext(r.Context())
	if r.Header.Get("accept") == "text/event-stream" {
		setStreamHeaders(w)
	}

	filters, err := params.parseFilters()
	if err != nil {
		log.Warn().Err(err).Msgf("invalid filter")
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := fmt.Sprintf("lsnr-handler-event %s from %s %s", handlerName, r.RemoteAddr, daemonctx.Uuid(r.Context()))
	if params.Filter != nil && len(*params.Filter) > 0 {
		name += " filters: [" + strings.Join(*params.Filter, " ") + "]"
	}
	AnnounceSub(bus, name)
	defer AnnounceUnSub(bus, name)

	sub := bus.Sub(name, pubsub.Timeout(time.Second))

	for _, filter := range filters {
		if kind, ok := filter.Kind.(event.Kinder); ok {
			log.Debug().Msgf("filtering %s %v", kind.Kind(), filter.Labels)
		} else {
			log.Warn().Msgf("skip filtering of %s %v", reflect.TypeOf(filter.Kind), filter.Labels)
			continue
		}
		sub.AddFilter(filter.Kind, filter.Labels...)
	}

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

// parseFilters return filters from *b.Filter
func (b *GetDaemonEventsParams) parseFilters() (filters []Filter, err error) {
	var filter Filter

	if b.Filter == nil {
		return
	}
	for _, s := range *b.Filter {
		filter, err = b.parseFilter(s)
		if err != nil {
			return
		}
		filters = append(filters, filter)
	}
	return
}

// parseFilter return filter from s
//
// filter syntax is: [kind][,label=value]*
func (b *GetDaemonEventsParams) parseFilter(s string) (filter Filter, err error) {
	for _, elem := range strings.Split(s, ",") {
		if strings.HasPrefix(elem, ".") {
			// TODO filter data ?
			continue
		}
		splitted := strings.Split(elem, "=")
		if len(splitted) == 1 {
			filter.Kind, err = msgbus.KindToT(splitted[0])
			if err != nil {
				return
			}
		} else if len(splitted) == 2 {
			filter.Labels = append(filter.Labels, pubsub.Label{splitted[0], splitted[1]})
		} else {
			err = errors.New("invalid filter expression: " + s)
			return
		}
	}
	return
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
