package nmon

import (
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/daemondata"
)

func (o *nmon) orchestrateDrained() {
	switch o.state.State {
	case cluster.NodeMonitorStateIdle:
		o.drainFreezeFromIdle()
	case cluster.NodeMonitorStateFrozen:
		o.drainFromIdle()
	}
}

func (o *nmon) drainFreezeFromIdle() {
	if d := daemondata.GetNodeStatus(o.dataCmdC, o.localhost); (d != nil) && !d.Frozen.IsZero() {
		// already frozen... advance to "frozen" state
		o.state.State = cluster.NodeMonitorStateFrozen
		o.updateIfChange()
		return
	}

	// freeze
	o.state.State = cluster.NodeMonitorStateFreezing
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run action freeze")
		if err := o.crmFreeze(); err != nil {
			o.cmdC <- cmdOrchestrate{state: cluster.NodeMonitorStateFreezing, newState: cluster.NodeMonitorStateFreezeFailed}
		} else {
			o.cmdC <- cmdOrchestrate{state: cluster.NodeMonitorStateFreezing, newState: cluster.NodeMonitorStateFrozen}
		}
	}()
	return
}

func (o *nmon) drainFromIdle() {
	o.state.State = cluster.NodeMonitorStateDraining
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run shutdown action on all local instances")
		if err := o.crmDrain(); err != nil {
			o.cmdC <- cmdOrchestrate{state: cluster.NodeMonitorStateDraining, newState: cluster.NodeMonitorStateDrainFailed}
		} else {
			o.cmdC <- cmdOrchestrate{state: cluster.NodeMonitorStateDraining, newState: cluster.NodeMonitorStateIdle}
		}
	}()
	return
}
