package imon

import "github.com/opensvc/om3/core/instance"

func (o *imon) orchestrateAborted() {
	o.log.Infof("abort orchestration: unset global expect")
	o.change = true
	o.state.GlobalExpect = instance.MonitorGlobalExpectNone
	o.state.GlobalExpectOptions = nil
	o.state.OrchestrationIsDone = true
	o.updateIfChange()
}
