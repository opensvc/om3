package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdObjectStop is the cobra flag set of the stop command.
	CmdObjectStop struct {
		flagSetGlobal
		flagSetObject
		flagSetAction
		flagSetAsync
		object.ActionOptionsStop
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectStop) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	t.flagSetGlobal.init(cmd)
	t.flagSetObject.init(cmd)
	t.flagSetAction.init(cmd)
	t.flagSetAsync.init(cmd)
	t.ActionOptionsStop.Init(cmd)
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
			ObjectSelector: mergeSelector(*selector, t.ObjectSelector, kind, ""),
			NodeSelector:   t.NodeSelector,
			Local:          t.Local,
			Action:         "stop",
			Target:         "stopped",
			Watch:          t.Watch,
			Format:         t.Format,
			Color:          t.Color,
		},
		Object: object.ObjectAction{
			Run: func(path object.Path) (interface{}, error) {
				intf := path.NewObject().(object.Starter)
				return nil, intf.Stop(t.ActionOptionsStop)
			},
		},
	}
	action.Do(a)
}
