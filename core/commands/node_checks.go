package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeChecks is the cobra flag set of the start command.
	CmdNodeChecks struct {
		OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeChecks) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdNodeChecks) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "checks",
		Short:   "Run the check drivers, push and print the instances",
		Aliases: []string{"check", "chec", "che", "ch"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *CmdNodeChecks) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("checks"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return n.Checks()
		}),
	).Do()
}
