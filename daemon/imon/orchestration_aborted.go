package imon

func (t *Manager) orchestrateAborted() {
	t.publishOrchestrationAborted()
	t.log.Infof("abort orchestration: unset global expect")
	t.change = true
	t.state.GlobalExpectOptions = nil
	t.state.OrchestrationIsDone = true
	t.updateIfChange()
}
