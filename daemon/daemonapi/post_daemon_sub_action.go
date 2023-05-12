package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonlogctx"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostDaemonSubAction(w http.ResponseWriter, r *http.Request) {
	log := daemonlogctx.Logger(r.Context()).With().Str("func", "PostDaemonSubAction").Logger()
	log.Debug().Msg("starting")

	var (
		payload api.PostDaemonSubAction
	)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		WriteProblemf(w, http.StatusBadRequest, "Invalid body", "%s", err)
		return
	}
	action := string(payload.Action)
	switch action {
	case "start":
	case "stop":
	default:
		WriteProblemf(w, http.StatusBadRequest, "Invalid body", "unexpected action: %s", action)
		return
	}
	var subs []string
	for _, sub := range payload.Subs {
		subs = append(subs, sub)
	}
	if len(subs) == 0 {
		WriteProblemf(w, http.StatusOK, "Daemon routine not found", "No daemon routine to %s", action)
		return
	}
	log.Info().Msgf("asking to %s sub components: %s", action, subs)
	bus := pubsub.BusFromContext(r.Context())
	for _, sub := range payload.Subs {
		log.Info().Msgf("ask to %s sub component: %s", action, sub)
		bus.Pub(&msgbus.DaemonCtl{Component: sub, Action: action}, pubsub.Label{"id", sub}, labelApi)
	}
	WriteProblemf(w, http.StatusOK, "daemon routines action queued", "%s %s", action, subs)
}
