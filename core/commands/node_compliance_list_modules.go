package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeComplianceListModules is the cobra flag set of the sysreport command.
	CmdNodeComplianceListModules struct {
		object.OptsNodeComplianceListModules
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeComplianceListModules) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsNodeComplianceListModules)
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
		nodeaction.WithLocal(t.Global.Local),
		nodeaction.WithRemoteNodes(t.Global.NodeSelector),
		nodeaction.WithFormat(t.Global.Format),
		nodeaction.WithColor(t.Global.Color),
		nodeaction.WithServer(t.Global.Server),
		nodeaction.WithRemoteAction("compliance list modules"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Global.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().ComplianceListModules(t.OptsNodeComplianceListModules)
		}),
	).Do()
}
