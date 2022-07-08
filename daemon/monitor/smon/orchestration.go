package smon

// orchestrate from svcagg vs global expect
func (o *smon) orchestrate() {
	if o.state.GlobalExpect == globalExpectUnset {
		// no expected status to reach
		return
	}

	switch o.state.GlobalExpect {
	case globalExpectStarted:
		o.orchestrateStarted()
	case globalExpectStopped:
		o.orchestrateStopped()
	case globalExpectFrozen:
		o.orchestrateFrozen()
	case globalExpectThawed:
		o.orchestrateThawed()
	}
	o.updateIfChange()
}
