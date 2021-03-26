package object

// OptsSet is the options of the Set object method.
type OptsSet struct {
	Global     OptsGlobal
	Lock       OptsLocking
	KeywordOps []string `flag:"kws"`
}

// Set gets a keyword value
func (t *Base) Set(options OptsSet) error {
	t.log.Error().Msg("not implemented")
	return nil
}
