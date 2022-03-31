package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeComplianceCheck is the cobra flag set of the sysreport command.
	CmdNodeComplianceCheck struct {
		object.OptsNodeComplianceCheck
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeComplianceCheck) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsNodeComplianceCheck)
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
		nodeaction.WithLocal(t.Global.Local),
		nodeaction.WithRemoteNodes(t.Global.NodeSelector),
		nodeaction.WithFormat(t.Global.Format),
		nodeaction.WithColor(t.Global.Color),
		nodeaction.WithServer(t.Global.Server),
		nodeaction.WithRemoteAction("compliance check"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format":    t.Global.Format,
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
