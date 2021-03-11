package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
)

type (
	CmdObjectUnfreeze struct {
		flagSetGlobal
		flagSetObject
		flagSetAsync
		flagSetAction
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectUnfreeze) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	t.flagSetGlobal.init(cmd)
	t.flagSetObject.init(cmd)
	t.flagSetAsync.init(cmd)
	t.flagSetAction.init(cmd)
}

func (t *CmdObjectUnfreeze) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "unfreeze",
		Short: "Unfreeze the selected objects",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectUnfreeze) run(selector *string, kind string) {
	a := action.ObjectAction{
		ObjectSelector: mergeSelector(*selector, t.ObjectSelector, kind, ""),
		NodeSelector:   t.NodeSelector,
		Local:          t.Local,
		Action:         "unfreeze",
		Method:         "Unfreeze",
		Target:         "frozen",
		Watch:          t.Watch,
		Format:         t.Format,
		Color:          t.Color,
	}
	action.Do(a)
}
