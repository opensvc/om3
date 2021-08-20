package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/commands"
)

var (
	nodeCmd = &cobra.Command{
		Use:   "node",
		Short: "Manage a opensvc cluster node",
	}
	nodePrintCmd = &cobra.Command{
		Use:   "print",
		Short: "Print node",
	}
	nodePushCmd = &cobra.Command{
		Use:   "push",
		Short: "Data pushing commands",
	}
	nodeScanCmd = &cobra.Command{
		Use:   "scan",
		Short: "Scan node",
	}

	cmdNodeChecks            commands.CmdNodeChecks
	cmdNodeLs                commands.NodeLs
	cmdNodePrintCapabilities commands.NodePrintCapabilities
	cmdNodePushAsset         commands.NodePushAsset
	cmdNodeScanCapabilities  commands.NodeScanCapabilities
)

func init() {
	root.AddCommand(nodeCmd)
	nodeCmd.AddCommand(nodePrintCmd)
	nodeCmd.AddCommand(nodePushCmd)
	nodeCmd.AddCommand(nodeScanCmd)

	cmdNodeChecks.Init(nodeCmd)
	cmdNodeLs.Init(nodeCmd)
	cmdNodePrintCapabilities.Init(nodePrintCmd)
	cmdNodePushAsset.Init(nodePushCmd)
	cmdNodeScanCapabilities.Init(nodeScanCmd)
}
