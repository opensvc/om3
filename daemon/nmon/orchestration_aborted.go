package nmon

import "github.com/opensvc/om3/core/node"

func (o *nmon) orchestrateAborted() {
	o.log.Info().Msg("abort orchestration: unset global expect")
	o.change = true
	o.state.GlobalExpect = node.MonitorGlobalExpectNone
	o.updateIfChange()
}
