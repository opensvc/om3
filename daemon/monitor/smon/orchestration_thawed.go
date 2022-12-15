package smon

func (o *smon) orchestrateThawed() {
	switch o.state.Status {
	case statusIdle:
		o.ThawedFromIdle()
	}
}

func (o *smon) ThawedFromIdle() {
	if o.thawedClearIfReached() {
		return
	}
	o.doTransitionAction(o.unfreeze, statusThawing, statusIdle, statusThawedFailed)
}

func (o *smon) thawedClearIfReached() bool {
	if o.instStatus[o.localhost].IsThawed() {
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
