package object

// DeleteSection removes sections from node config
func (t Node) DeleteSection(s ...string) error {
	return t.config.DeleteSections(s...)
}
