package smon

func (o *smon) orchestratePlacedAt(dst string) {
	if !o.acceptPlacedAtOrchestration(dst) {
		o.log.Warn().Msgf("no solution for orchestrate placed at %s", dst)
		return
	}
	switch o.state.Status {
	default:
		o.log.Error().Msgf("don't know how to orchestrate placed at %s from %s", dst, o.state.Status)
	}
}

//func (o *smon) stoppedFromThawed() {
//	o.doAction(o.crmFreeze, statusFreezing, statusIdle, statusFreezeFailed)
//}

func (o *smon) acceptPlacedAtOrchestration(dst string) bool {
	return true
}
