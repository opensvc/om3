package object

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/check"
)

// ActionOptionsNodeChecks is the options of the Checks function.
type ActionOptionsNodeChecks struct {
	ActionOptionsGlobal
}

// Init declares the cobra flags associated with the type options.
func (t *ActionOptionsNodeChecks) Init(cmd *cobra.Command) {
	t.ActionOptionsGlobal.init(cmd)
}

// Checks find and runs the check drivers.
func (t Node) Checks(options ActionOptionsNodeChecks) check.ResultSet {
	rs := check.Runner{}.Do()
	return *rs
}
