package smon

func (o *smon) orchestrateFrozen() {
	switch o.state.Status {
	case statusIdle:
		o.frozenFromIdle()
	}
}

func (o *smon) frozenFromIdle() {
	if o.frozenClearIfReached() {
		return
	}
	o.doTransitionAction(o.freeze, statusFreezing, statusIdle, statusFreezeFailed)
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
