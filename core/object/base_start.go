package object

import (
	"time"

	"github.com/spf13/cobra"
)

// ActionOptionsStart is the options of the Start object method.
type ActionOptionsStart struct {
	ActionOptionsGlobal
	ActionOptionsLocking
	ActionOptionsResources
	ActionOptionsForce
}

// Init declares the cobra flags associated with the type options
func (t *ActionOptionsStart) Init(cmd *cobra.Command) {
	t.ActionOptionsGlobal.init(cmd)
	t.ActionOptionsLocking.init(cmd)
	t.ActionOptionsResources.init(cmd)
	t.ActionOptionsForce.init(cmd)
}

// Start starts the local instance of the object
func (t *Base) Start(options ActionOptionsStart) error {
	lock, err := t.Lock("", options.LockTimeout, "start")
	if err != nil {
		return err
	}
	defer lock.Unlock()
	time.Sleep(10 * time.Second)
	return nil
}
