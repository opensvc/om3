package discover

import (
	"time"

	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/omon"
	"github.com/opensvc/om3/util/pubsub"
)

func (d *discover) omon(started chan<- bool) {
	log := d.log.With().Str("func", "omon").Logger()
	log.Info().Msg("started")
	bus := pubsub.BusFromContext(d.ctx)
	sub := bus.Sub("omon-from-cfg-create")
	sub.AddFilter(msgbus.ConfigUpdated{})
	sub.Start()
	started <- true
	defer func() {
		defer log.Info().Msg("stopped")
		if err := sub.Stop(); err != nil {
			d.log.Error().Err(err).Msg("subscription stop")
		}
		tC := time.After(d.dropCmdDuration)
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
			case msgbus.ConfigUpdated:
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
