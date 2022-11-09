package nmon

import (
	"opensvc.com/opensvc/daemon/daemondata"
)

func (o *nmon) orchestrateFrozen() {
	if !o.isConvergedGlobalExpect() {
		return
	}
	switch o.state.Status {
	case statusIdle:
		o.frozenFromIdle()
	}
}

func (o *nmon) frozenFromIdle() {
	if o.frozenClearIfReached() {
		return
	}
	o.state.Status = statusFreezing
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run action freeze")
		if err := o.crmFreeze(); err != nil {
			o.cmdC <- cmdOrchestrate{state: statusFreezing, newState: statusFreezeFailed}
		} else {
			o.cmdC <- cmdOrchestrate{state: statusFreezing, newState: statusIdle}
		}
	}()
	return
}

func (o *nmon) frozenClearIfReached() bool {
	if d := daemondata.GetNodeStatus(o.dataCmdC, o.localhost); (d != nil) && !d.Frozen.IsZero() {
		o.log.Info().Msg("local status is frozen, unset global expect")
		o.change = true
		o.state.GlobalExpect = globalExpectUnset
		o.clearPending()
		return true
	}
	return false
}
