package object

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/config"
)

// PrintConfigOptions is the options of the PrintConfig object method.
type ActionOptionsPrintConfig struct {
	ActionOptionsGlobal
	ActionOptionsLocking
	ActionOptionsEval
	ActionOptionsImpersonate
}

// Init declares the cobra flags associated with the type options
func (t *ActionOptionsPrintConfig) Init(cmd *cobra.Command) {
	t.ActionOptionsGlobal.init(cmd)
	t.ActionOptionsLocking.init(cmd)
	t.ActionOptionsEval.init(cmd)
	t.ActionOptionsImpersonate.init(cmd)
}

// PrintConfig gets a keyword value
func (t *Base) PrintConfig(options ActionOptionsPrintConfig) (config.Raw, error) {
	if options.Eval {
		// TODO
		return config.Raw{}, nil
	} else {
		return t.config.Raw(), nil
	}
}
