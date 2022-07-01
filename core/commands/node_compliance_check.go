package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeComplianceCheck is the cobra flag set of the sysreport command.
	CmdNodeComplianceCheck struct {
		OptsGlobal
		object.OptsNodeComplianceCheck
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeComplianceCheck) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdNodeComplianceCheck) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "check",
		Short:   "run compliance checks",
		Aliases: []string{"chec", "che", "ch"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *CmdNodeComplianceCheck) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("compliance check"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format":    t.Format,
			"force":     t.Force,
			"module":    t.Module,
			"moduleset": t.Moduleset,
			"attach":    t.Attach,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().ComplianceCheck(t.OptsNodeComplianceCheck)
		}),
	).Do()
}
