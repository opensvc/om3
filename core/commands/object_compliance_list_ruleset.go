package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectComplianceListRuleset is the cobra flag set of the sysreport command.
	CmdObjectComplianceListRuleset struct {
		OptsGlobal
		object.OptsObjectComplianceListRuleset
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectComplianceListRuleset) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectComplianceListRuleset) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "ruleset",
		Short:   "List compliance ruleset available to this node.",
		Aliases: []string{"rulese", "rules", "rule", "rul", "ru"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectComplianceListRuleset) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithServer(t.Server),
		objectaction.WithRemoteAction("compliance env"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Format,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			if o, err := object.NewSvc(p); err != nil {
				return nil, err
			} else {
				return o.ComplianceListRuleset(t.OptsObjectComplianceListRuleset)
			}
		}),
	).Do()
}
