package smon

import "strings"

// orchestrate from svcagg vs global expect
func (o *smon) orchestrate() {
	if o.state.GlobalExpect == globalExpectUnset {
		// no expected status to reach
		return
	}
	if !o.isConvergedGlobalExpect() {
		return
	}

	switch o.state.GlobalExpect {
	case globalExpectFrozen:
		o.orchestrateFrozen()
	case globalExpectProvisioned:
		o.orchestrateProvisioned()
	case globalExpectPlaced:
		o.orchestratePlaced()
	case globalExpectPlacedAt:
		o.orchestratePlacedAt("")
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
	default:
		if strings.HasPrefix(o.state.GlobalExpect, globalExpectPlacedAt) {
			o.orchestratePlacedAt(o.state.GlobalExpect[len(globalExpectPlacedAt):])
		}
	}
	o.updateIfChange()
}
