package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeChecks is the cobra flag set of the start command.
	CmdNodeChecks struct {
		object.OptsNodeChecks
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeChecks) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	object.InstallFlags(cmd, &t.OptsNodeChecks)
}

func (t *CmdNodeChecks) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "checks",
		Short:   "Run the check drivers, push and print the instances",
		Aliases: []string{"check", "chec", "che", "ch"},
		Run: func(cmd *cobra.Command, args []string) {
			t.run()
		},
	}
}

func (t *CmdNodeChecks) run() {
	a := action.NodeAction{
		Action: action.Action{
			NodeSelector: t.Global.NodeSelector,
			Local:        t.Global.Local,
			Action:       "checks",
			PostFlags: map[string]interface{}{
				"format": t.Global.Format,
			},
			Format: t.Global.Format,
			Color:  t.Global.Color,
		},
		Node: object.NodeAction{
			Run: func() (interface{}, error) {
				return object.NewNode().Checks(t.OptsNodeChecks), nil
			},
		},
	}
	action.Do(a)
}
