package nmon

import (
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/daemondata"
)

func (o *nmon) orchestrateDrained() {
	switch o.state.Status {
	case cluster.NodeMonitorStatusIdle:
		o.drainFreezeFromIdle()
	case cluster.NodeMonitorStatusFrozen:
		o.drainFromIdle()
	}
}

func (o *nmon) drainFreezeFromIdle() {
	if d := daemondata.GetNodeStatus(o.dataCmdC, o.localhost); (d != nil) && !d.Frozen.IsZero() {
		// already frozen... advance to "frozen" state
		o.state.Status = cluster.NodeMonitorStatusFrozen
		o.updateIfChange()
		return
	}

	// freeze
	o.state.Status = cluster.NodeMonitorStatusFreezing
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run action freeze")
		if err := o.crmFreeze(); err != nil {
			o.cmdC <- cmdOrchestrate{state: cluster.NodeMonitorStatusFreezing, newState: cluster.NodeMonitorStatusFreezeFailed}
		} else {
			o.cmdC <- cmdOrchestrate{state: cluster.NodeMonitorStatusFreezing, newState: cluster.NodeMonitorStatusFrozen}
		}
	}()
	return
}

func (o *nmon) drainFromIdle() {
	o.state.Status = cluster.NodeMonitorStatusDraining
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run shutdown action on all local instances")
		if err := o.crmDrain(); err != nil {
			o.cmdC <- cmdOrchestrate{state: cluster.NodeMonitorStatusDraining, newState: cluster.NodeMonitorStatusDrainFailed}
		} else {
			o.cmdC <- cmdOrchestrate{state: cluster.NodeMonitorStatusDraining, newState: cluster.NodeMonitorStatusIdle}
		}
	}()
	return
}
