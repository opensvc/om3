package object

import (
	"time"

	"github.com/spf13/cobra"
)

// ActionOptionsStop is the options of the Stop object method.
type ActionOptionsStop struct {
	ActionOptionsGlobal
	ActionOptionsLocking
	ActionOptionsResources
	ActionOptionsForce
}

// Init declares the cobra flags associated with the type options
func (t *ActionOptionsStop) Init(cmd *cobra.Command) {
	t.ActionOptionsGlobal.init(cmd)
	t.ActionOptionsLocking.init(cmd)
	t.ActionOptionsResources.init(cmd)
	t.ActionOptionsForce.init(cmd)
}

// Stop starts the local instance of the object
func (t *Base) Stop(options ActionOptionsStop) error {
	lock, err := t.Lock("", options.LockTimeout, "stop")
	if err != nil {
		return err
	}
	defer lock.Unlock()
	time.Sleep(10 * time.Second)
	return nil
}
