package smon

// orchestrate from svcagg vs global expect
func (o *smon) orchestrate() {
	if o.state.GlobalExpect == globalExpectUnset {
		// no expected status to reach
		return
	}

	switch o.state.GlobalExpect {
	case globalExpectFrozen:
		o.orchestrateFrozen()
	case globalExpectProvisioned:
		o.orchestrateProvisioned()
	case globalExpectPurged:
		o.orchestratePurged()
	case globalExpectStarted:
		o.orchestrateStarted()
	case globalExpectStopped:
		o.orchestrateStopped()
	case globalExpectThawed:
		o.orchestrateThawed()
	case globalExpectUnProvisioned:
		o.orchestrateUnProvisioned()
	case globalExpectAborted:
		o.orchestrateAborted()
	}
	o.updateIfChange()
}
