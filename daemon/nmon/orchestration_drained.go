package nmon

import "opensvc.com/opensvc/core/node"

func (o *nmon) orchestrateDrained() {
	switch o.state.State {
	case node.MonitorStateIdle:
		o.drainFreezeFromIdle()
	case node.MonitorStateFrozen:
		o.drainFromIdle()
	}
}

func (o *nmon) drainFreezeFromIdle() {
	if d := o.databus.GetNodeStatus(o.localhost); (d != nil) && !d.Frozen.IsZero() {
		// already frozen... advance to "frozen" state
		o.state.State = node.MonitorStateFrozen
		o.updateIfChange()
		return
	}

	// freeze
	o.state.State = node.MonitorStateFreezing
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run action freeze")
		if err := o.crmFreeze(); err != nil {
			o.cmdC <- cmdOrchestrate{state: node.MonitorStateFreezing, newState: node.MonitorStateFreezeFailed}
		} else {
			o.cmdC <- cmdOrchestrate{state: node.MonitorStateFreezing, newState: node.MonitorStateFrozen}
		}
	}()
	return
}

func (o *nmon) drainFromIdle() {
	o.state.State = node.MonitorStateDraining
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run shutdown action on all local instances")
		if err := o.crmDrain(); err != nil {
			o.cmdC <- cmdOrchestrate{state: node.MonitorStateDraining, newState: node.MonitorStateDrainFailed}
		} else {
			o.cmdC <- cmdOrchestrate{state: node.MonitorStateDraining, newState: node.MonitorStateIdle}
		}
	}()
	return
}
