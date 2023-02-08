package daemonapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/daemon/daemonlogctx"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostDaemonSubAction(w http.ResponseWriter, r *http.Request) {
	log := daemonlogctx.Logger(r.Context()).With().Str("func", "PostDaemonSubAction").Logger()
	log.Debug().Msg("starting")

	var (
		payload PostDaemonSubAction
	)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	action := string(payload.Action)
	switch action {
	case "start":
	case "stop":
	default:
		sendError(w, http.StatusBadRequest, "unexpected action: "+action)
		return
	}
	var subs []string
	for _, sub := range payload.Subs {
		subs = append(subs, sub)
	}
	if len(subs) == 0 {
		w.WriteHeader(http.StatusOK)
		msg := fmt.Sprintf("empty component list to %s", action)
		_ = json.NewEncoder(w).Encode(ResponseText(msg))
		return
	}
	log.Info().Msgf("asking to %s sub components: %s", action, subs)
	bus := pubsub.BusFromContext(r.Context())
	for _, sub := range payload.Subs {
		log.Info().Msgf("ask to %s sub component: %s", action, sub)
		bus.Pub(msgbus.DaemonCtl{Component: sub, Action: action}, pubsub.Label{"id", sub}, labelApi)
	}
	w.WriteHeader(http.StatusOK)
	msg := fmt.Sprintf("ask to %s sub components: %s", action, subs)
	_ = json.NewEncoder(w).Encode(ResponseText(msg))
}
