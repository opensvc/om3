package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdObjectStart is the cobra flag set of the start command.
	CmdObjectStart struct {
		flagSetGlobal
		flagSetObject
		flagSetAction
		flagSetAsync
		object.ActionOptionsStart
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectStart) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	t.flagSetGlobal.init(cmd)
	t.flagSetObject.init(cmd)
	t.flagSetAction.init(cmd)
	t.flagSetAsync.init(cmd)
	t.ActionOptionsStart.Init(cmd)
}

func (t *CmdObjectStart) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the selected objects",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectStart) run(selector *string, kind string) {
	a := action.ObjectAction{
		Action: action.Action{
			ObjectSelector: mergeSelector(*selector, t.ObjectSelector, kind, ""),
			NodeSelector:   t.NodeSelector,
			Local:          t.Local,
			Action:         "start",
			Target:         "started",
			Watch:          t.Watch,
			Format:         t.Format,
			Color:          t.Color,
		},
		Object: object.ObjectAction{
			Run: func(path object.Path) (interface{}, error) {
				return nil, path.NewObject().(object.Starter).Start(t.ActionOptionsStart)
			},
		},
	}
	action.Do(a)
}
