package smon

import (
	"context"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/object"
)

func (o *smon) cmdTryLeaveReady(ctx context.Context) {
	select {
	case <-ctx.Done():
		switch ctx.Err() {
		case context.Canceled:
			o.cancelReady = nil
			return
		case context.DeadlineExceeded:
			if o.state.Status != statusReady {
				o.log.Error().Msgf("cmdTryLeaveReady with non ready status: %s", o.state.Status)
				o.tryCancelReady()
				return
			}
			status, ok := o.instStatus[o.localhost]
			if !ok {
				o.log.Error().Msg("cmdTryLeaveReady but local instance status is unset")
				o.tryCancelReady()
				return
			}
			o.tryCancelReady()
			o.unsetStatusWhenReached(status)
			o.updateIfChange()
			o.orchestrateStartedFromReady()
			o.updateIfChange()
		}
	default:
		o.log.Error().Msg("cmdTryLeaveReady on not done context")
	}
}

// cmdSvcAggUpdated updateIfChange state global expect from aggregated status
func (o *smon) cmdSvcAggUpdated(v object.AggregatedStatus) {
	o.svcAgg = v
	o.unsetStatusWhenReached(o.instStatus[o.localhost])
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func (o *smon) cmdSetSmonClient(c instance.Monitor) {
	if c.GlobalExpect != o.state.GlobalExpect {
		o.log.Info().Msgf("client request global expect to %s", c.GlobalExpect)
		o.change = true
		o.state.GlobalExpect = c.GlobalExpect
		o.updateIfChange()
	}
	o.unsetStatusWhenReached(o.instStatus[o.localhost])
	o.updateIfChange()
	o.orchestrate()
	o.updateIfChange()
}

func (o *smon) tryCancelReady() {
	if o.cancelReady != nil {
		o.cancelReady()
		o.cancelReady = nil
	}
}
