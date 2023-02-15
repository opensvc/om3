package nmon

import "github.com/opensvc/om3/core/node"

func (o *nmon) orchestrateAborted() {
	o.log.Info().Msg("abort orchestration: unset global expect")
	o.change = true
	o.state.GlobalExpect = node.MonitorGlobalExpectNone

	// drained is abortable
	switch o.state.LocalExpect {
	case node.MonitorLocalExpectDrained:
		o.state.LocalExpect = node.MonitorLocalExpectNone
	}

	o.updateIfChange()
}
