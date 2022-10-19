package daemonapi

import (
	"encoding/json"
	"net/http"

	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemonlogctx"
)

func (a *DaemonApi) PostDaemonStop(w http.ResponseWriter, r *http.Request) {
	log := daemonlogctx.Logger(r.Context()).With().Str("func", "PostDaemonStop").Logger()
	log.Debug().Msg("starting")

	daemon := daemonctx.Daemon(r.Context())
	if daemon.Running() {
		log.Info().Msg("daemon stopping")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ResponseText("daemon stopping"))
		go func() {
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
