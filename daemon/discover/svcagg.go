package discover

import (
	"time"

	"opensvc.com/opensvc/daemon/monitor/omon"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/pubsub"
)

func (d *discover) omon(started chan<- bool) {
	log := d.log.With().Str("func", "omon").Logger()
	log.Info().Msg("started")
	bus := pubsub.BusFromContext(d.ctx)
	sub := bus.Sub("omon-from-cfg-create")
	sub.AddFilter(msgbus.CfgUpdated{})
	sub.Start()
	defer sub.Stop()
	started <- true
	defer func() {
		defer log.Info().Msg("stopped")
		tC := time.After(dropCmdTimeout)
		for {
			select {
			case <-tC:
				return
			case <-d.objectMonitorCmdC:
			}
		}
	}()
	for {
		select {
		case <-d.ctx.Done():
			return
		case i := <-sub.C:
			switch c := i.(type) {
			case msgbus.CfgUpdated:
				s := c.Path.String()
				if _, ok := d.objectMonitor[s]; !ok {
					log.Info().Msgf("discover new object %s", s)
					if err := omon.Start(d.ctx, c.Path, c.Value, d.objectMonitorCmdC); err != nil {
						log.Error().Err(err).Msgf("omon.Start %s", s)
						return
					}
					d.objectMonitor[s] = make(map[string]struct{})
				}
			}
		case i := <-d.objectMonitorCmdC:
			switch c := i.(type) {
			case msgbus.ObjectStatusDone:
				delete(d.objectMonitor, c.Path.String())
			default:
				log.Error().Interface("cmd", i).Msg("unexpected cmd")
			}
		}
	}
}
