package nmon

// orchestrate from svcagg vs global expect
func (o *nmon) orchestrate() {
	if o.state.GlobalExpect == globalExpectUnset {
		// no expected status to reach
		return
	}

	switch o.state.GlobalExpect {
	case globalExpectFrozen:
		o.orchestrateFrozen()
	case globalExpectThawed:
		o.orchestrateThawed()
	}
	o.updateIfChange()
}
