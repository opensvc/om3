package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeComplianceEnv is the cobra flag set of the sysreport command.
	CmdNodeComplianceEnv struct {
		object.OptsNodeComplianceEnv
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeComplianceEnv) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsNodeComplianceEnv)
}

func (t *CmdNodeComplianceEnv) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "env",
		Short:   "Show the environment variables set during a compliance module run.",
		Aliases: []string{"en"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *CmdNodeComplianceEnv) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Global.Local),
		nodeaction.WithRemoteNodes(t.Global.NodeSelector),
		nodeaction.WithFormat(t.Global.Format),
		nodeaction.WithColor(t.Global.Color),
		nodeaction.WithServer(t.Global.Server),
		nodeaction.WithRemoteAction("compliance env"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format":    t.Global.Format,
			"moduleset": t.Moduleset,
			"module":    t.Module,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().ComplianceEnv(t.OptsNodeComplianceEnv)
		}),
	).Do()
}
