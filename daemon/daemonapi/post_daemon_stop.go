package daemonapi

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
)

func (a *DaemonApi) PostDaemonStop(ctx echo.Context) error {
	log := LogHandler(ctx, "PostDaemonStop")
	log.Debug().Msg("starting")

	maintenance := func() {
		log.Info().Msg("announce maintenance state")
		state := node.MonitorStateMaintenance
		a.EventBus.Pub(&msgbus.SetNodeMonitor{
			Node: hostname.Hostname(),
			Value: node.MonitorUpdate{
				State: &state,
			},
		}, labelApi)
		time.Sleep(2 * daemondata.PropagationInterval())
	}

	if a.Daemon.Running() {
		maintenance()

		log.Info().Msg("daemon stopping")
		go func() {
			// Give time for response received by client before stop daemon
			time.Sleep(50 * time.Millisecond)
			if err := a.Daemon.Stop(); err != nil {
				log.Error().Err(err).Msg("daemon stop failure")
				return
			}
			log.Info().Msg("daemon stopped")
		}()
		return JSONProblem(ctx, http.StatusOK, "Daemon stopping", "")
	} else {
		return JSONProblem(ctx, http.StatusOK, "Daemon is already stopping", "")
	}
}
