package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeComplianceFixable is the cobra flag set of the sysreport command.
	CmdNodeComplianceFixable struct {
		object.OptsNodeComplianceFixable
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeComplianceFixable) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsNodeComplianceFixable)
}

func (t *CmdNodeComplianceFixable) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "fixable",
		Short:   "run compliance fixable-tests",
		Aliases: []string{"fixabl", "fixab", "fixa"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *CmdNodeComplianceFixable) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Global.Local),
		nodeaction.WithRemoteNodes(t.Global.NodeSelector),
		nodeaction.WithFormat(t.Global.Format),
		nodeaction.WithColor(t.Global.Color),
		nodeaction.WithServer(t.Global.Server),
		nodeaction.WithRemoteAction("compliance fixable"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format":    t.Global.Format,
			"force":     t.Force,
			"module":    t.Module,
			"moduleset": t.Moduleset,
			"attach":    t.Attach,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().ComplianceFixable(t.OptsNodeComplianceFixable)
		}),
	).Do()
}
