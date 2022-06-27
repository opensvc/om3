package smon

import "time"

// convergeGlobalExpectFromRemote set global expect from most recent global expect value
func (o *smon) convergeGlobalExpectFromRemote() {
	var mostRecentNode string
	var mostRecentUpdated time.Time
	for node, instMon := range o.instSmon {
		nodeTime := instMon.GlobalExpectUpdated.Time()
		if mostRecentUpdated.Before(nodeTime) {
			mostRecentNode = node
			mostRecentUpdated = nodeTime
		}
	}
	if o.state.GlobalExpectUpdated.Time().Before(mostRecentUpdated) {
		o.change = true
		o.state.GlobalExpect = o.instSmon[mostRecentNode].GlobalExpect
		o.state.GlobalExpectUpdated = o.instSmon[mostRecentNode].GlobalExpectUpdated
		strVal := o.instSmon[mostRecentNode].GlobalExpect
		if strVal == "" {
			strVal = "unset"
		}
		o.log.Info().Msgf("apply global expect change from remote %s %s %s",
			mostRecentNode, strVal, mostRecentUpdated)
	}
}
