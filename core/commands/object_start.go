package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdObjectStart is the cobra flag set of the start command.
	CmdObjectStart struct {
		object.OptsStart
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectStart) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	object.InstallFlags(cmd, t)
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
			ObjectSelector: mergeSelector(*selector, t.Global.ObjectSelector, kind, ""),
			NodeSelector:   t.Global.NodeSelector,
			Local:          t.Global.Local,
			Action:         "start",
			Target:         "started",
			Watch:          t.Async.Watch,
			Format:         t.Global.Format,
			Color:          t.Global.Color,
		},
		Object: object.Action{
			Run: func(path object.Path) (interface{}, error) {
				return nil, path.NewObject().(object.Starter).Start(t.OptsStart)
			},
		},
	}
	action.Do(a)
}
