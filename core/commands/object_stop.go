package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdObjectStop is the cobra flag set of the stop command.
	CmdObjectStop struct {
		object.OptsStop
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectStop) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	object.InstallFlags(cmd, t)
}

func (t *CmdObjectStop) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the selected objects",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectStop) run(selector *string, kind string) {
	a := action.ObjectAction{
		Action: action.Action{
			ObjectSelector: mergeSelector(*selector, t.Global.ObjectSelector, kind, ""),
			NodeSelector:   t.Global.NodeSelector,
			Local:          t.Global.Local,
			Action:         "stop",
			Target:         "stopped",
			Watch:          t.Async.Watch,
			Format:         t.Global.Format,
			Color:          t.Global.Color,
		},
		Object: object.Action{
			Run: func(path object.Path) (interface{}, error) {
				intf := path.NewObject().(object.Starter)
				return nil, intf.Stop(t.OptsStop)
			},
		},
	}
	action.Do(a)
}
