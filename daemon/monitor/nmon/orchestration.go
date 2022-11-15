package nmon

// orchestrate from svcagg vs global expect
func (o *nmon) orchestrate() {
	switch o.state.LocalExpect {
	case localExpectUnset:
	case localExpectDrained:
		o.orchestrateDrained()
	}

	switch o.state.GlobalExpect {
	case globalExpectUnset:
	case globalExpectAborted:
		o.orchestrateAborted()
	case globalExpectFrozen:
		o.orchestrateFrozen()
	case globalExpectThawed:
		o.orchestrateThawed()
	}
	o.updateIfChange()
}
