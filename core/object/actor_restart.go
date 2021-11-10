package object

// Restart stops then starts the local instance of the object
func (t *Base) Restart(options OptsStart) error {
	if err := t.Stop(OptsStop{
		OptsGlobal:           options.OptsGlobal,
		OptsAsync:            options.OptsAsync,
		OptsLocking:          options.OptsLocking,
		OptsResourceSelector: options.OptsResourceSelector,
		OptTo:                options.OptTo,
		OptForce:             options.OptForce,
	}); err != nil {
		return err
	}
	return t.Start(options)
}
