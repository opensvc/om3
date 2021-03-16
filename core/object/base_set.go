package object

import "github.com/spf13/cobra"

// ActionOptionsSet is the options of the Set object method.
type ActionOptionsSet struct {
	ActionOptionsGlobal
	ActionOptionsLocking
	ActionOptionsKeywordOps
}

// Init declares the cobra flags associated with the type options
func (t *ActionOptionsSet) Init(cmd *cobra.Command) {
	t.ActionOptionsGlobal.init(cmd)
	t.ActionOptionsLocking.init(cmd)
	t.ActionOptionsKeywordOps.init(cmd)
}

// Set gets a keyword value
func (t *Base) Set(options ActionOptionsSet) error {
	t.log.Error().Msg("not implemented")
	return nil
}
