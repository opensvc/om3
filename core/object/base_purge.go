package object

//
// Purge is the 'purge' object action entrypoint.
//
// This function behaves like a 'delete --unprovision'.
//
func (t Base) Purge() error {
	return t.Delete(OptsDelete{Unprovision: true})
}
