package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeComplianceShowRuleset is the cobra flag set of the sysreport command.
	CmdNodeComplianceShowRuleset struct {
		OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeComplianceShowRuleset) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdNodeComplianceShowRuleset) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ruleset",
		Short:   "Show compliance rules applying to this node.",
		Aliases: []string{"rulese", "rules", "rule", "rul", "ru"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *CmdNodeComplianceShowRuleset) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("compliance show ruleset"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			comp, err := n.NewCompliance()
			if err != nil {
				return nil, err
			}
			return comp.GetRulesets()
		}),
	).Do()
}
