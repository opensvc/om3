package object

import (
	"opensvc.com/opensvc/core/check"
)

// OptsNodeChecks is the options of the Checks function.
type OptsNodeChecks struct {
	Global OptsGlobal
}

// Checks find and runs the check drivers.
func (t Node) Checks(options OptsNodeChecks) check.ResultSet {
	rs := check.Runner{}.Do()
	return *rs
}
