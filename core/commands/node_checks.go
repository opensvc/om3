package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/object"

	_ "opensvc.com/opensvc/drivers/check/fs_i/df"
	_ "opensvc.com/opensvc/drivers/check/fs_u/df"
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
	nodeaction.New(
		nodeaction.WithLocal(t.Global.Local),
		nodeaction.WithRemoteNodes(t.Global.NodeSelector),
		nodeaction.WithFormat(t.Global.Format),
		nodeaction.WithColor(t.Global.Color),
		nodeaction.WithServer(t.Global.Server),
		nodeaction.WithRemoteAction("checks"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Global.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().Checks(t.OptsNodeChecks), nil
		}),
	).Do()
}
