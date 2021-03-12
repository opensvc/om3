package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
	"opensvc.com/opensvc/core/object"
)

type (
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
		ObjectSelector: mergeSelector(*selector, t.ObjectSelector, kind, ""),
		NodeSelector:   t.NodeSelector,
		Local:          t.Local,
		Action:         "start",
		Method:         "Start",
		MethodArgs:     []interface{}{t.ActionOptionsStart},
		Target:         "started",
		Watch:          t.Watch,
		Format:         t.Format,
		Color:          t.Color,
	}
	action.Do(a)
}
