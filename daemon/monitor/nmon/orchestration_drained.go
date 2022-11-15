package nmon

import (
	"opensvc.com/opensvc/daemon/daemondata"
)

func (o *nmon) orchestrateDrained() {
	switch o.state.Status {
	case statusIdle:
		o.drainFreezeFromIdle()
	case statusFrozen:
		o.drainFromIdle()
	}
}

func (o *nmon) drainFreezeFromIdle() {
	if d := daemondata.GetNodeStatus(o.dataCmdC, o.localhost); (d != nil) && !d.Frozen.IsZero() {
		// already frozen... advance to "frozen" state
		o.state.Status = statusFrozen
		o.updateIfChange()
		return
	}

	// freeze
	o.state.Status = statusFreezing
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run action freeze")
		if err := o.crmFreeze(); err != nil {
			o.cmdC <- cmdOrchestrate{state: statusFreezing, newState: statusFreezeFailed}
		} else {
			o.cmdC <- cmdOrchestrate{state: statusFreezing, newState: statusFrozen}
		}
	}()
	return
}

func (o *nmon) drainFromIdle() {
	o.state.Status = statusDraining
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run shutdown action on all local instances")
		if err := o.crmDrain(); err != nil {
			o.cmdC <- cmdOrchestrate{state: statusDraining, newState: statusDrainFailed}
		} else {
			o.cmdC <- cmdOrchestrate{state: statusDraining, newState: statusIdle}
		}
	}()
	return
}
