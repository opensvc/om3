package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
)

type (
	CmdObjectSet struct {
		flagSetGlobal
		flagSetObject
		flagSetAction
		Keywords []string
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectSet) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	t.flagSetGlobal.init(cmd)
	t.flagSetObject.init(cmd)
	t.flagSetAction.init(cmd)
	cmd.Flags().StringSliceVar(&t.Keywords, "kw", []string{}, "A keyword to set (operators = += |= -= ^=)")
}

func (t *CmdObjectSet) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "set",
		Short: "Set a configuration key raw value",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectSet) run(selector *string, kind string) {
	a := action.ObjectAction{
		ObjectSelector: mergeSelector(*selector, t.ObjectSelector, kind, ""),
		NodeSelector:   t.NodeSelector,
		Local:          t.Local,
		DefaultIsLocal: true,
		Action:         "set",
		Method:         "Set",
		MethodArgs: []interface{}{
			t.Keywords,
		},
		Flags: map[string]interface{}{
			"kw": t.Keywords,
		},
		Format: t.Format,
		Color:  t.Color,
	}
	action.Do(a)
}
