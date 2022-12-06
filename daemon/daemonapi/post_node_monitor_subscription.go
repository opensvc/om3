package daemonapi

import (
	"context"
	"net/http"
	"time"

	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/pubsub"
)

// PostNodeMonitorSubscription creates new subscription for node monitor Events in rss format.
// TODO: Honor grants
func (a *DaemonApi) PostNodeMonitorSubscription(w http.ResponseWriter, r *http.Request, params PostNodeMonitorSubscriptionParams) {
	var (
		handlerName = "PostNodeMonitorSubscription"
		limit       int64
		maxDuration = 5 * time.Second
	)
	log := getLogger(r, handlerName)
	log.Debug().Msg("starting")
	defer log.Debug().Msg("done")
	if params.Limit != nil {
		limit = *params.Limit
	}
	if params.Duration != nil {
		if v, err := converters.Duration.Convert(*params.Duration); err != nil {
			log.Info().Err(err).Msgf("invalid duration: %s", *params.Duration)
			sendError(w, http.StatusBadRequest, "invalid duration")
		} else {
			maxDuration = *v.(*time.Duration)
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), maxDuration)
	defer cancel()
	bus := pubsub.BusFromContext(ctx)
	if r.Header.Get("accept") == "text/event-stream" {
		setStreamHeaders(w)
	}
	w.WriteHeader(http.StatusOK)

	name := handlerName + " from " + r.RemoteAddr + " " + daemonctx.Uuid(r.Context()).String()
	sub := bus.SubWithTimeout(name, time.Second)
	sub.AddFilter(msgbus.NodeMonitorUpdated{})
	sub.Start()
	defer sub.Stop()
	err := <-writeEvents(ctx, w, sub.C, limit)
	if err != nil {
		log.Error().Err(err).Msgf("write events")
	}
}
