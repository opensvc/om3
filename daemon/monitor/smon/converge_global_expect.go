package smon

import (
	"time"
)

// convergeGlobalExpectFromRemote set global expect from most recent global expect value
func (o *smon) convergeGlobalExpectFromRemote() {
	var mostRecentNode string
	var mostRecentUpdated time.Time
	for node, instMon := range o.instSmon {
		if instMon.GlobalExpect == "" {
			// converge "aborted" to unset via orchestration
			continue
		}
		nodeTime := instMon.GlobalExpectUpdated
		if mostRecentUpdated.Before(nodeTime) {
			mostRecentNode = node
			mostRecentUpdated = nodeTime
		}
	}
	if mostRecentUpdated.IsZero() {
		return
	}
	if mostRecentUpdated.After(o.state.GlobalExpectUpdated) {
		o.change = true
		o.state.GlobalExpect = o.instSmon[mostRecentNode].GlobalExpect
		o.state.GlobalExpectUpdated = o.instSmon[mostRecentNode].GlobalExpectUpdated
		strVal := o.instSmon[mostRecentNode].GlobalExpect
		if strVal == "" {
			strVal = "unset"
		}
		o.log.Info().Msgf("# apply global expect change from remote %s -> %s %s",
			mostRecentNode, strVal, mostRecentUpdated)
	}
}

func (o *smon) isConvergedGlobalExpect() bool {
	localUpdated := o.state.GlobalExpectUpdated
	for s, v := range o.instSmon {
		if s == o.localhost {
			continue
		}
		if localUpdated.After(v.GlobalExpectUpdated) {
			o.log.Debug().Msgf("wait GlobalExpect propagation on %s", s)
			return false
		}
	}
	return true
}
