package smon

import (
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
)

func (o *smon) orchestratePurged() {
	if !o.isConvergedGlobalExpect() {
		return
	}
	switch o.state.Status {
	case statusDeleted:
		o.purgedFromDeleted()
	case statusIdle:
		o.purgedFromIdle()
	case statusUnProvisioned:
		o.purgedFromUnProvisioned()
	}
}

func (o *smon) purgedFromIdle() {
	if o.instStatus[o.localhost].Avail == status.Up {
		o.purgedFromIdleUp()
		return
	}
	if o.instStatus[o.localhost].Provisioned.Bool() {
		o.purgedFromIdleProvisioned()
		return
	}
	go func() {
		o.cmdC <- moncmd.New(cmdOrchestrate{state: statusIdle, newState: statusUnProvisioned})
	}()
	return
}

func (o *smon) purgedFromDeleted() {
	o.change = true
	o.state.GlobalExpect = globalExpectUnset
	o.state.Status = statusIdle
	o.updateIfChange()
}

func (o *smon) purgedFromUnProvisioned() {
	o.change = true
	o.state.Status = statusDeleting
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run action delete")
		if err := o.crmDelete(); err != nil {
			o.cmdC <- moncmd.New(cmdOrchestrate{state: statusDeleting, newState: statusPurgeFailed})
		} else {
			o.cmdC <- moncmd.New(cmdOrchestrate{state: statusDeleting, newState: statusDeleted})
		}
	}()
	return
}

func (o *smon) purgedFromIdleUp() {
	o.change = true
	o.state.Status = statusStopping
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run action stop")
		if err := o.crmStop(); err != nil {
			o.cmdC <- moncmd.New(cmdOrchestrate{state: statusStopping, newState: statusStopFailed})
		} else {
			o.cmdC <- moncmd.New(cmdOrchestrate{state: statusStopping, newState: statusIdle})
		}
	}()
	return
}

func (o *smon) purgedFromIdleProvisioned() {
	o.change = true
	o.state.Status = statusUnProvisioning
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run action unprovision")
		if err := o.crmUnprovisionLeader(); err != nil {
			o.cmdC <- moncmd.New(cmdOrchestrate{state: statusUnProvisioning, newState: statusPurgeFailed})
		} else {
			o.cmdC <- moncmd.New(cmdOrchestrate{state: statusUnProvisioning, newState: statusUnProvisioned})
		}
	}()
	return
}
