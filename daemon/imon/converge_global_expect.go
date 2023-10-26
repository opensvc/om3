package imon

import (
	"fmt"
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
		nodeTime := instMon.GlobalExpectUpdatedAt
		if mostRecentUpdated.Before(nodeTime) {
			mostRecentNode = node
			mostRecentUpdated = nodeTime
		}
	}
	if mostRecentUpdated.IsZero() {
		return
	}
	if mostRecentUpdated.After(o.state.GlobalExpectUpdatedAt) {
		o.change = true
		o.state.GlobalExpect = o.instMonitor[mostRecentNode].GlobalExpect
		o.state.GlobalExpectUpdatedAt = o.instMonitor[mostRecentNode].GlobalExpectUpdatedAt
		o.state.GlobalExpectOptions = o.instMonitor[mostRecentNode].GlobalExpectOptions
		o.state.OrchestrationId = o.instMonitor[mostRecentNode].OrchestrationId
		o.state.State = instance.MonitorStateIdle
		strVal := o.instMonitor[mostRecentNode].GlobalExpect.String()
		if strVal == "" {
			strVal = "unset"
		}
		o.log.Infof("fetch global expect from node %s -> %s orchestration id %s updated at %s",
			mostRecentNode, strVal, o.state.OrchestrationId, mostRecentUpdated)
	}
}

func (o *imon) isConvergedGlobalExpect() bool {
	localUpdated := o.state.GlobalExpectUpdatedAt
	for s, v := range o.instMonitor {
		if s == o.localhost {
			err := fmt.Errorf("bug: isConvergedGlobalExpect detect unexpected localhost in internal instance monitor cache keys")
			o.log.Errorf("isConvergedGlobalExpect: %s", err)
			panic(err)
		}
		if localUpdated.After(v.GlobalExpectUpdatedAt) {
			o.log.Debugf("wait GlobalExpect propagation on %s", s)
			return false
		}
	}
	return true
}
