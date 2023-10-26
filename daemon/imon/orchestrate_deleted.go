package imon

import (
	"github.com/opensvc/om3/core/instance"
)

func (o *imon) orchestrateDeleted() {
	o.log.Debugf("orchestrateDeleted starting from %s", o.state.State)
	switch o.state.State {
	case instance.MonitorStateDeleted:
		o.deletedFromDeleted()
	case instance.MonitorStateIdle,
		instance.MonitorStateBootFailed,
		instance.MonitorStateFreezeFailed,
		instance.MonitorStateProvisionFailed,
		instance.MonitorStateStartFailed,
		instance.MonitorStateStopFailed,
		instance.MonitorStateThawedFailed,
		instance.MonitorStateUnprovisionFailed:
		o.deletedFromIdle()
	case instance.MonitorStateWaitChildren:
		o.deletedFromWaitChildren()
	case instance.MonitorStateDeleting:
	default:
		o.log.Warnf("orchestrateDeleted has no solution from state %s", o.state.State)
	}
}

func (o *imon) deletedFromIdle() {
	if o.setWaitChildren() {
		return
	}
	o.doAction(o.crmDelete, instance.MonitorStateDeleting, instance.MonitorStateDeleted, instance.MonitorStateDeleteFailed)
	return
}

func (o *imon) deletedFromDeleted() {
	o.log.Warnf("have been deleted, we should die soon")
}

func (o *imon) deletedFromWaitChildren() {
	if o.setWaitChildren() {
		return
	}
	o.doAction(o.crmDelete, instance.MonitorStateDeleting, instance.MonitorStateDeleted, instance.MonitorStateDeleteFailed)
	return
}
