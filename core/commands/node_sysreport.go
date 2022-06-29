package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeSysreport is the cobra flag set of the sysreport command.
	CmdNodeSysreport struct {
		OptsGlobal
		object.OptsNodeSysreport
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeSysreport) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdNodeSysreport) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "sysreport",
		Short:   "Push system report to the collector for archiving and diff analysis. The --force option resend all monitored files and outputs to the collector instead of only those that changed since the last sysreport.",
		Aliases: []string{"sysrepor", "sysrepo", "sysrep", "sysre", "sysr", "sys", "sy"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *CmdNodeSysreport) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("sysreport"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Format,
			"force":  t.Force,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return nil, object.NewNode().Sysreport(t.OptsNodeSysreport)
		}),
	).Do()
}
