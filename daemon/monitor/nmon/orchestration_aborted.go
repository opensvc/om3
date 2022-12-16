package nmon

import "opensvc.com/opensvc/core/cluster"

func (o *nmon) orchestrateAborted() {
	o.log.Info().Msg("abort orchestration: unset global expect")
	o.change = true
	o.state.GlobalExpect = cluster.NodeMonitorGlobalExpectUnset
	o.updateIfChange()
}
