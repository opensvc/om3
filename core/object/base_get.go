package object

import (
	"github.com/spf13/cobra"
)

// ActionOptionsGet is the options of the Get object method.
type ActionOptionsGet struct {
	ActionOptionsGlobal
	ActionOptionsLocking
	ActionOptionsKeyword
	ActionOptionsEval
}

// Init declares the cobra flags associated with the type options
func (t *ActionOptionsGet) Init(cmd *cobra.Command) {
	t.ActionOptionsGlobal.init(cmd)
	t.ActionOptionsLocking.init(cmd)
	t.ActionOptionsKeyword.init(cmd)
	t.ActionOptionsEval.init(cmd)
}

// Get gets a keyword value
func (t *Base) Get(options ActionOptionsGet) (interface{}, error) {
	if options.Eval {
		return t.config.Eval(options.Keyword)
	} else {
		return t.config.Get(options.Keyword)
	}
}
