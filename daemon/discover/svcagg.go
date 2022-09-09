package discover

import (
	"time"

	"opensvc.com/opensvc/daemon/daemonps"
	ps "opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/monitor/svcagg"
	"opensvc.com/opensvc/util/pubsub"
)

func (d *discover) agg() {
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
	defer ps.UnSub(bus, ps.SubCfg(bus, pubsub.OpUpdate, "agg-from-cfg-create", "", d.onEvAgg))
	for {
		select {
		case <-d.ctx.Done():
			log.Info().Msg("stopped")
			return
		case i := <-d.svcaggCmdC:
			switch c := (*i).(type) {
			case daemonps.MonSvcAggDone:
				delete(d.svcAgg, c.Path.String())
			case daemonps.CfgUpdated:
				s := c.Path.String()
				if _, ok := d.svcAgg[s]; !ok {
					log.Info().Msgf("discover new object %s", s)
					if err := svcagg.Start(d.ctx, c.Path, c.Config, d.svcaggCmdC); err != nil {
						log.Error().Err(err).Msgf("svcAgg.Start %s", s)
						return
					}
					d.svcAgg[s] = make(map[string]struct{})
				}
			default:
				log.Error().Interface("cmd", i).Msg("unexpected cmd")
			}
		}
	}
}

func (d *discover) onEvAgg(i interface{}) {
	select {
	case <-d.ctx.Done():
	case d.svcaggCmdC <- daemonps.NewMsg(i):
	}
}
