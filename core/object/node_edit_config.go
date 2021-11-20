package object

func (t Node) EditConfig(opts OptsEditConfig) error {
	return editConfig(t.ConfigFile(), opts)
}
