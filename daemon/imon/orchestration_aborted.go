package imon

import "github.com/opensvc/om3/core/instance"

func (o *imon) orchestrateAborted() {
	o.log.Info().Msgf("daemon: imon: %s: abort orchestration: unset global expect", o.path)
	o.change = true
	o.state.GlobalExpect = instance.MonitorGlobalExpectNone
	o.state.GlobalExpectOptions = nil
	o.state.OrchestrationIsDone = true
	o.updateIfChange()
}
