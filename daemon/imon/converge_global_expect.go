package imon

import (
	"time"

	"github.com/opensvc/om3/core/instance"
)

// convergeGlobalExpectFromRemote set global expect from most recent global expect value
func (o *imon) convergeGlobalExpectFromRemote() {
	var mostRecentNode string
	var mostRecentUpdated time.Time
	for node, instMon := range o.instMonitor {
		if instMon.GlobalExpect == instance.MonitorGlobalExpectZero {
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
		o.state.GlobalExpect = o.instMonitor[mostRecentNode].GlobalExpect
		o.state.GlobalExpectUpdated = o.instMonitor[mostRecentNode].GlobalExpectUpdated
		o.state.GlobalExpectOptions = o.instMonitor[mostRecentNode].GlobalExpectOptions
		o.state.OrchestrationId = o.instMonitor[mostRecentNode].OrchestrationId
		strVal := o.instMonitor[mostRecentNode].GlobalExpect.String()
		if strVal == "" {
			strVal = "unset"
		}
		o.log.Info().Msgf("# apply global expect change from remote %s -> %s %s",
			mostRecentNode, strVal, mostRecentUpdated)
	}
}

func (o *imon) isConvergedGlobalExpect() bool {
	localUpdated := o.state.GlobalExpectUpdated
	for s, v := range o.instMonitor {
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
