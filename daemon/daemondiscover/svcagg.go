package daemondiscover

import (
	"time"

	"opensvc.com/opensvc/daemon/daemonctx"
	ps "opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/daemon/monitor/svcagg"
	"opensvc.com/opensvc/util/pubsub"
)

func (d *discover) svcaggRoutine() {
	d.log.Info().Msg("svcaggRoutine started")
	defer func() {
		done := time.After(dropCmdTimeout)
		for {
			select {
			case <-done:
				return
			case <-d.svcaggCmdC:
			}
		}
	}()
	c := daemonctx.DaemonPubSubCmd(d.ctx)
	defer ps.UnSub(c, ps.SubCfg(c, pubsub.OpUpdate, "svc-discover-agg-from-cfg-create", "", d.onEvAgg))
	for {
		select {
		case <-d.ctx.Done():
			d.log.Info().Msg("svcagg routine done")
		case i := <-d.svcaggCmdC:
			switch c := (*i).(type) {
			case moncmd.MonSvcAggDone:
				delete(d.svcAgg, c.Path.String())
			case moncmd.CfgUpdated:
				s := c.Path.String()
				d.log.Info().Msgf("svcaggRoutine detect moncmd.CfgUpdated %s", s)
				if _, ok := d.svcAgg[s]; !ok {
					d.log.Info().Msgf("svcaggRoutine creating new svcagg %s", s)
					if err := svcagg.Start(d.ctx, c.Path, c.Config, d.svcaggCmdC); err != nil {
						d.log.Error().Err(err).Msgf("svcAgg.Start %s", s)
						return
					}
					d.svcAgg[s] = make(map[string]struct{})
				}
			default:
				d.log.Error().Interface("cmd", i).Msg("svcagg routine unexpected cmd")
			}
		}
	}
}

func (d *discover) onEvAgg(i interface{}) {
	d.svcaggCmdC <- moncmd.New(i)
}
