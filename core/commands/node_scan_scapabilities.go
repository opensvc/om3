package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeScanCapabilities is the cobra flag set of the node scan command.
	CmdNodeScanCapabilities struct {
		object.OptsNodeScanCapabilities
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeScanCapabilities) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsNodeScanCapabilities)
}

var (
	scanLong = `Scan the node for capabilities.

Capabilities are normally scanned at daemon startup and when the installed 
system packages change, so admins only have to use this when they want manually 
installed software to be discovered without restarting the daemon.`
)

func (t *CmdNodeScanCapabilities) cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "capabilities",
		Short: "scan the node for capabilities",
		Long:  scanLong,
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *CmdNodeScanCapabilities) run() {
	nodeaction.New(
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().NodeScanCapabilities()
		}),
	).Do()
}
