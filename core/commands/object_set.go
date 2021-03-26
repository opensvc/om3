package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdObjectSet is the cobra flag set of the set command.
	CmdObjectSet struct {
		object.OptsSet
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectSet) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	object.InstallFlags(cmd, t)
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
		Action: action.Action{
			ObjectSelector: mergeSelector(*selector, t.Global.ObjectSelector, kind, ""),
			NodeSelector:   t.Global.NodeSelector,
			Local:          t.Global.Local,
			DefaultIsLocal: true,
			Action:         "set",
			Flags: map[string]interface{}{
				"kw": t.KeywordOps,
			},
			Format: t.Global.Format,
			Color:  t.Global.Color,
		},
		Object: object.Action{
			Run: func(path object.Path) (interface{}, error) {
				return nil, path.NewObject().(object.Configurer).Set(t.OptsSet)
			},
		},
	}
	action.Do(a)
}
