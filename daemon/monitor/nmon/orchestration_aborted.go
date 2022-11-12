package nmon

func (o *nmon) orchestrateAborted() {
	o.log.Info().Msg("abort orchestration: unset global expect")
	o.change = true
	o.state.GlobalExpect = globalExpectUnset
	o.updateIfChange()
}
