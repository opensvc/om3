package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeComplianceListModules is the cobra flag set of the sysreport command.
	CmdNodeComplianceListModules struct {
		OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeComplianceListModules) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdNodeComplianceListModules) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "modules",
		Short:   "List modules available on this node.",
		Aliases: []string{"module", "modul", "modu", "mod"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *CmdNodeComplianceListModules) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("compliance list modules"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().ComplianceListModules()
		}),
	).Do()
}
