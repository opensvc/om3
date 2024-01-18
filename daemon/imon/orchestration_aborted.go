package imon

import "github.com/opensvc/om3/core/instance"

func (t *Manager) orchestrateAborted() {
	t.log.Infof("abort orchestration: unset global expect")
	t.change = true
	t.state.GlobalExpect = instance.MonitorGlobalExpectNone
	t.state.GlobalExpectOptions = nil
	t.state.OrchestrationIsDone = true
	t.updateIfChange()
}
