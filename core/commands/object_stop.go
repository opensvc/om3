package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
)

type (
	CmdObjectStop struct {
		flagSetGlobal
		flagSetObject
		flagSetAction
		flagSetAsync
		Force bool
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
	cmd.Flags().BoolVar(&t.Force, "force", false, "allow dangerous operations")
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
		ObjectSelector: mergeSelector(*selector, t.ObjectSelector, kind, ""),
		NodeSelector:   t.NodeSelector,
		Local:          t.Local,
		Action:         "stop",
		Method:         "Stop",
		Flags:          t,
		Target:         "stopped",
		Watch:          t.Watch,
		Format:         t.Format,
		Color:          t.Color,
	}
	action.Do(a)
}
