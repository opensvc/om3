package smon

func (o *smon) orchestrateThawed() {
	if !o.isConvergedGlobalExpect() {
		return
	}
	switch o.state.Status {
	case statusIdle:
		o.ThawedFromIdle()
	}
}

func (o *smon) ThawedFromIdle() {
	if o.thawedClearIfReached() {
		return
	}
	o.state.Status = statusThawing
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run action unfreeze")
		if err := o.crmUnfreeze(); err != nil {
			o.cmdC <- cmdOrchestrate{state: statusThawing, newState: statusThawedFailed}
		} else {
			o.cmdC <- cmdOrchestrate{state: statusThawing, newState: statusIdle}
		}
	}()
	return
}

func (o *smon) thawedClearIfReached() bool {
	if o.instStatus[o.localhost].Frozen.IsZero() {
		o.log.Info().Msg("local status is thawed, unset global expect")
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
