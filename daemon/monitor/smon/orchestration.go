package smon

import (
	"strings"
)

// orchestrate from svcagg vs global expect
func (o *smon) orchestrate() {
	if _, ok := o.instStatus[o.localhost]; !ok {
		return
	}
	if _, ok := o.nodeStatus[o.localhost]; !ok {
		return
	}
	if !o.isConvergedGlobalExpect() {
		return
	}

	switch o.state.GlobalExpect {
	case globalExpectUnset:
		o.orchestrateHA()
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
	case globalExpectUnprovisioned:
		o.orchestrateUnprovisioned()
	case globalExpectAborted:
		o.orchestrateAborted()
	default:
		if strings.HasPrefix(o.state.GlobalExpect, globalExpectPlacedAt) {
			o.orchestratePlacedAt(o.state.GlobalExpect[len(globalExpectPlacedAt):])
		}
	}
	o.updateIfChange()
}
