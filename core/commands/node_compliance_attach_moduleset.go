package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeComplianceAttachModuleset is the cobra flag set of the sysreport command.
	CmdNodeComplianceAttachModuleset struct {
		object.OptsNodeComplianceAttachModuleset
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeComplianceAttachModuleset) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsNodeComplianceAttachModuleset)
}

func (t *CmdNodeComplianceAttachModuleset) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "moduleset",
		Short:   "attach compliance moduleset to this node.",
		Long:    "modules of attached modulesets are checked on schedule.",
		Aliases: []string{"modulese", "modules", "module", "modul", "modu", "mod", "mo"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *CmdNodeComplianceAttachModuleset) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Global.Local),
		nodeaction.WithRemoteNodes(t.Global.NodeSelector),
		nodeaction.WithFormat(t.Global.Format),
		nodeaction.WithColor(t.Global.Color),
		nodeaction.WithServer(t.Global.Server),
		nodeaction.WithRemoteAction("compliance attach moduleset"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Global.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().ComplianceAttachModuleset(t.OptsNodeComplianceAttachModuleset)
		}),
	).Do()
}
