package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeComplianceListModuleset is the cobra flag set of the sysreport command.
	CmdNodeComplianceListModuleset struct {
		OptsGlobal
		object.OptsNodeComplianceListModuleset
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeComplianceListModuleset) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdNodeComplianceListModuleset) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "moduleset",
		Short:   "List compliance moduleset available to this node.",
		Aliases: []string{"modulese"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *CmdNodeComplianceListModuleset) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("compliance list modulesets"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().ComplianceListModuleset(t.OptsNodeComplianceListModuleset)
		}),
	).Do()
}
