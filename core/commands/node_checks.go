package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeChecks is the cobra flag set of the start command.
	CmdNodeChecks struct {
		flagSetGlobal
		flagSetAction
		object.ActionOptionsNodeChecks
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeChecks) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	t.flagSetGlobal.init(cmd)
	t.flagSetAction.init(cmd)
	t.ActionOptionsNodeChecks.Init(cmd)
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
			NodeSelector: t.NodeSelector,
			Local:        t.Local,
			Action:       "checks",
			PostFlags: map[string]interface{}{
				"format": t.Format,
			},
			Format: t.Format,
			Color:  t.Color,
		},
		Node: object.NodeAction{
			Run: func() (interface{}, error) {
				opts := object.ActionOptionsNodeChecks{}
				return object.NewNode().Checks(opts), nil
			},
		},
	}
	action.Do(a)
}
