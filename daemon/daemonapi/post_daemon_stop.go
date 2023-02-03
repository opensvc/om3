package daemonapi

import (
	"encoding/json"
	"net/http"
	"time"

	"opensvc.com/opensvc/core/node"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

func (a *DaemonApi) PostDaemonStop(w http.ResponseWriter, r *http.Request) {
	log := daemonlogctx.Logger(r.Context()).With().Str("func", "PostDaemonStop").Logger()
	log.Debug().Msg("starting")

	ctx := r.Context()
	daemon := daemonctx.Daemon(ctx)

	maintenance := func() {
		log.Info().Msg("announce maintenance state")
		bus := pubsub.BusFromContext(ctx)
		state := node.MonitorStateMaintenance
		bus.Pub(msgbus.SetNodeMonitor{
			Node: hostname.Hostname(),
			Value: node.MonitorUpdate{
				State: &state,
			},
		}, labelApi)
		time.Sleep(2 * daemondata.PropagationInterval())
	}

	if daemon.Running() {
		maintenance()

		log.Info().Msg("daemon stopping")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ResponseText("daemon stopping"))
		go func() {
			// Give time for response received by client before stop daemon
			time.Sleep(50 * time.Millisecond)
			if err := daemon.Stop(); err != nil {
				log.Error().Err(err).Msg("daemon stop failure")
				return
			}
			log.Info().Msg("daemon stopped")
		}()
	} else {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ResponseText("no daemon to stop"))
	}
}
