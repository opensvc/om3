package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeComplianceFixable is the cobra flag set of the sysreport command.
	CmdNodeComplianceFixable struct {
		OptsGlobal
		object.OptsNodeComplianceFixable
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeComplianceFixable) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
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
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("compliance fixable"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format":    t.Format,
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
