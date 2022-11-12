package nmon

import (
	"time"
)

// convergeGlobalExpectFromRemote set global expect from most recent global expect value
func (o *nmon) convergeGlobalExpectFromRemote() {
	var mostRecentNode string
	var mostRecentUpdated time.Time
	for node, data := range o.nmons {
		if data.GlobalExpect == "" {
			// converge "aborted" to unset via orchestration
			continue
		}
		nodeTime := data.GlobalExpectUpdated
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
		o.state.GlobalExpect = o.nmons[mostRecentNode].GlobalExpect
		o.state.GlobalExpectUpdated = o.nmons[mostRecentNode].GlobalExpectUpdated
		strVal := o.nmons[mostRecentNode].GlobalExpect
		if strVal == "" {
			strVal = "unset"
		}
		o.log.Info().Msgf("apply global expect change from remote %s -> %s %s",
			mostRecentNode, strVal, mostRecentUpdated)
	}
}

func (o *nmon) isConvergedGlobalExpect() bool {
	localUpdated := o.state.GlobalExpectUpdated
	for s, data := range o.nmons {
		if s == o.localhost {
			continue
		}
		if localUpdated.After(data.GlobalExpectUpdated) {
			o.log.Debug().Msgf("wait GlobalExpect propagation on %s", s)
			return false
		}
	}
	return true
}
