package object

//
// Purge is the 'purge' object action entrypoint.
//
// This function behaves like a 'delete --unprovision'.
//
func (t core) Purge() error {
	return t.Delete(OptsDelete{Unprovision: true})
}
