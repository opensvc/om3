package object

// Restart stops then starts the local instance of the object
func (t *core) Restart(options OptsStart) error {
	if err := t.Stop(OptsStop{
		OptsLock:             options.OptsLock,
		OptsResourceSelector: options.OptsResourceSelector,
		OptTo:                options.OptTo,
		OptForce:             options.OptForce,
		OptDryRun:            options.OptDryRun,
	}); err != nil {
		return err
	}
	return t.Start(options)
}
