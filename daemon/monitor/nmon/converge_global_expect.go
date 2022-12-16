package nmon

import (
	"time"

	"opensvc.com/opensvc/core/cluster"
)

// convergeGlobalExpectFromRemote set global expect from most recent global expect value
func (o *nmon) convergeGlobalExpectFromRemote() {
	var mostRecentNode string
	var mostRecentUpdated time.Time
	for node, data := range o.nodeMonitor {
		if data.GlobalExpect == cluster.NodeMonitorGlobalExpectUnset {
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
		o.state.GlobalExpect = o.nodeMonitor[mostRecentNode].GlobalExpect
		o.state.GlobalExpectUpdated = o.nodeMonitor[mostRecentNode].GlobalExpectUpdated
		strVal := o.nodeMonitor[mostRecentNode].GlobalExpect.String()
		if strVal == "" {
			strVal = "unset"
		}
		o.log.Info().Msgf("# apply global expect change from remote %s -> %s %s",
			mostRecentNode, strVal, mostRecentUpdated)
	}
}

func (o *nmon) isConvergedGlobalExpect() bool {
	localUpdated := o.state.GlobalExpectUpdated
	for s, data := range o.nodeMonitor {
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
