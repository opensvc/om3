package object

import "github.com/spf13/cobra"

// ActionOptionsStart is the options of the Start object method.
type ActionOptionsStatus struct {
	ActionOptionsGlobal
	ActionOptionsLocking
	ActionOptionsRefresh
}

// Init declares the cobra flags associated with the type options
func (t *ActionOptionsStatus) Init(cmd *cobra.Command) {
	t.ActionOptionsGlobal.init(cmd)
	t.ActionOptionsLocking.init(cmd)
	t.ActionOptionsRefresh.init(cmd)
}

// Status returns the service status dataset
func (t *Base) Status(options ActionOptionsStatus) (interface{}, error) {
	return nil, nil
}
