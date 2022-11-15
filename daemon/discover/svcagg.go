package discover

import (
	"time"

	"opensvc.com/opensvc/daemon/monitor/svcagg"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/pubsub"
)

func (d *discover) agg(started chan<- bool) {
	log := d.log.With().Str("func", "agg").Logger()
	log.Info().Msg("started")
	defer func() {
		t := time.NewTicker(dropCmdTimeout)
		defer t.Stop()
		for {
			select {
			case <-d.ctx.Done():
				return
			case <-t.C:
				return
			case <-d.svcaggCmdC:
			}
		}
	}()
	bus := pubsub.BusFromContext(d.ctx)
	sub := bus.Sub("agg-from-cfg-create")
	sub.AddFilter(msgbus.CfgUpdated{})
	sub.Start()
	defer sub.Stop()
	started <- true
	for {
		select {
		case <-d.ctx.Done():
			log.Info().Msg("stopped")
			return
		case i := <-sub.C:
			switch c := i.(type) {
			case msgbus.CfgUpdated:
				s := c.Path.String()
				if _, ok := d.svcAgg[s]; !ok {
					log.Info().Msgf("discover new object %s", s)
					if err := svcagg.Start(d.ctx, c.Path, c.Config, d.svcaggCmdC); err != nil {
						log.Error().Err(err).Msgf("svcAgg.Start %s", s)
						return
					}
					d.svcAgg[s] = make(map[string]struct{})
				}
			}
		case i := <-d.svcaggCmdC:
			switch c := i.(type) {
			case msgbus.ObjectAggDone:
				delete(d.svcAgg, c.Path.String())
			default:
				log.Error().Interface("cmd", i).Msg("unexpected cmd")
			}
		}
	}
}
