package nmon

import (
	"time"

	"github.com/opensvc/om3/core/node"
)

// convergeGlobalExpectFromRemote set global expect from most recent global expect value
func (o *nmon) convergeGlobalExpectFromRemote() {
	var mostRecentNode string
	var mostRecentUpdated time.Time
	for nodename, data := range o.nodeMonitor {
		if data.GlobalExpect == node.MonitorGlobalExpectZero {
			continue
		}
		if data.GlobalExpect == node.MonitorGlobalExpectNone {
			continue
		}
		nodeTime := data.GlobalExpectUpdatedAt
		if mostRecentUpdated.Before(nodeTime) {
			mostRecentNode = nodename
			mostRecentUpdated = nodeTime
		}
	}
	if mostRecentUpdated.IsZero() {
		return
	}
	if mostRecentUpdated.After(o.state.GlobalExpectUpdatedAt) {
		o.change = true
		o.state.GlobalExpect = o.nodeMonitor[mostRecentNode].GlobalExpect
		o.state.GlobalExpectUpdatedAt = o.nodeMonitor[mostRecentNode].GlobalExpectUpdatedAt
		strVal := o.nodeMonitor[mostRecentNode].GlobalExpect.String()
		if strVal == "" {
			strVal = "unset"
		}
		o.log.Infof("fetch global expect from node %s -> %s updated at %s", mostRecentNode, strVal, mostRecentUpdated)
	}
}

func (o *nmon) isConvergedGlobalExpect() bool {
	localUpdated := o.state.GlobalExpectUpdatedAt
	for s, data := range o.nodeMonitor {
		if s == o.localhost {
			continue
		}
		if localUpdated.After(data.GlobalExpectUpdatedAt) {
			o.log.Debugf("wait global expect propagation on %s", s)
			return false
		}
	}
	return true
}
