package smon

import "opensvc.com/opensvc/daemon/daemonps"

func (o *smon) orchestrateFrozen() {
	if !o.isConvergedGlobalExpect() {
		return
	}
	switch o.state.Status {
	case statusIdle:
		o.frozenFromIdle()
	}
}

func (o *smon) frozenFromIdle() {
	if o.frozenClearIfReached() {
		return
	}
	o.state.Status = statusFreezing
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run action freeze")
		if err := o.crmFreeze(); err != nil {
			o.cmdC <- daemonps.NewMsg(cmdOrchestrate{state: statusFreezing, newState: statusFreezeFailed})
		} else {
			o.cmdC <- daemonps.NewMsg(cmdOrchestrate{state: statusFreezing, newState: statusIdle})
		}
	}()
	return
}

func (o *smon) frozenClearIfReached() bool {
	if !o.instStatus[o.localhost].Frozen.IsZero() {
		o.log.Info().Msg("local status is frozen, unset global expect")
		o.change = true
		o.state.GlobalExpect = globalExpectUnset
		if o.state.LocalExpect != statusIdle {
			o.state.LocalExpect = statusIdle
		}
		o.clearPending()
		return true
	}
	return false
}
