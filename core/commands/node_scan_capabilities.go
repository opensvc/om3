package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// NodeScanCapabilities is the cobra flag set of the node scan command.
	NodeScanCapabilities struct {
		Global object.OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodeScanCapabilities) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.Global)
}

func (t *NodeScanCapabilities) cmd() *cobra.Command {
	long := `Scan the node for capabilities.

Capabilities are normally scanned at daemon startup and when the installed 
system packages change, so admins only have to use this when they want manually 
installed software to be discovered without restarting the daemon.`

	return &cobra.Command{
		Use:     "capabilities",
		Short:   "scan the node for capabilities",
		Aliases: []string{"capa", "caps", "cap", "ca", "c"},
		Long:    long,
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodeScanCapabilities) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Global.Local),
		nodeaction.WithRemoteNodes(t.Global.NodeSelector),
		nodeaction.WithFormat(t.Global.Format),
		nodeaction.WithColor(t.Global.Color),
		nodeaction.WithServer(t.Global.Server),
		nodeaction.WithRemoteAction("scan capabilities"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Global.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().NodeScanCapabilities()
		}),
	).Do()
}
