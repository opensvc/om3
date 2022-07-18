package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeComplianceDetachRuleset is the cobra flag set of the sysreport command.
	CmdNodeComplianceDetachRuleset struct {
		OptsGlobal
		OptRuleset
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeComplianceDetachRuleset) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdNodeComplianceDetachRuleset) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ruleset",
		Short:   "detach compliance ruleset from this node.",
		Long:    "rules of attached rulesets are made available to their module.",
		Aliases: []string{"rulese", "rules", "rule", "rul", "ru"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *CmdNodeComplianceDetachRuleset) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("compliance detach ruleset"),
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
			return nil, comp.DetachRuleset(t.Ruleset)
		}),
	).Do()
}
