package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdObjectGet is the cobra flag set of the get command.
	CmdObjectGet struct {
		object.OptsGet
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectGet) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	object.InstallFlags(cmd, &t.OptsGet)
}

func (t *CmdObjectGet) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Get a configuration key value.",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectGet) run(selector *string, kind string) {
	a := action.ObjectAction{
		Action: action.Action{
			ObjectSelector: mergeSelector(*selector, t.Global.ObjectSelector, kind, ""),
			NodeSelector:   t.Global.NodeSelector,
			Local:          t.Global.Local,
			DefaultIsLocal: true,
			Action:         "get",
			Flags: map[string]interface{}{
				"kw": t.Keyword,
			},
			Format: t.Global.Format,
			Color:  t.Global.Color,
		},
		Object: object.Action{
			Run: func(path object.Path) (interface{}, error) {
				return path.NewObject().(object.Configurer).Get(t.OptsGet)
			},
		},
	}
	action.Do(a)
}
