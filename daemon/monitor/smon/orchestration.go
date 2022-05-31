package smon

import (
	"context"

	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
)

// orchestrate from svcagg vs global expect
func (o *smon) orchestrate() {
	if o.state.Status != statusIdle {
		return
	}
	if o.state.GlobalExpect == globalExpectUnset {
		// no expected status to reach
		return
	}

	switch o.state.GlobalExpect {
	case globalExpectStarted:
		o.orchestrateStarted()
	case globalExpectStopped:
		o.orchestrateStopped()
	}
}

func (o *smon) orchestrateStopped() {
	switch o.svcAgg.Avail {
	case status.Down:
		o.change = true
		o.state.GlobalExpect = globalExpectUnset
		return
	default:
		localInstanceStatus, ok := o.instStatus[o.localhost]
		if !ok {
			return
		}
		switch localInstanceStatus.Avail {
		case status.Up:
			o.change = true
			o.state.Status = statusStarting
			go func() {
				o.log.Info().Msg("run action stop")
			}()
		}
	}
}

func (o *smon) orchestrateStartedFromReady() {
	if o.state.GlobalExpect == globalExpectStarted {
		o.orchestrateStarted()
	}
}

func (o *smon) orchestrateStarted() {
	o.log.Info().Msgf("orchestrateStarted from %s", o.svcAgg.Avail)
	switch o.svcAgg.Avail {
	case status.Down:
		switch o.state.Status {
		case statusIdle:
			if o.hasOtherNodeActing() {
				o.log.Info().Msg("hasOtherNodeActing")
				return
			}
			if o.cancelReady != nil {
				o.log.Error().Msg("bug orchestrateStarted from idle but cancel ready exists")
				o.cancelReady()
				o.cancelReady = nil
				return
			}
			o.change = true
			o.state.Status = statusReady
			readyCtx, cancel := context.WithTimeout(o.ctx, readyDuration)
			o.cancelReady = cancel
			go func() {
				select {
				case <-readyCtx.Done():
					if readyCtx.Err() == context.Canceled {
						return
					}
					go func() {
						o.cmdC <- moncmd.New(cmdReady{ctx: readyCtx})
					}()
					return
				}
			}()
			return
		case statusReady:
			o.change = true
			if o.hasOtherNodeActing() {
				o.log.Info().Msg("other found greater solution, abandon ready")
				o.state.Status = statusIdle
				if o.cancelReady != nil {
					o.cancelReady()
					o.cancelReady = nil
				}
				return
			}
			o.state.Status = statusStarting
			go func() {
				o.log.Info().Msg("run action start")
			}()
			return
		default:
			o.log.Info().Msgf("orchestrateStarted from status no action: %s", o.svcAgg.Avail)
		}
	}
}
