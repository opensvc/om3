package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/commands"
)

var (
	nodeCmd = &cobra.Command{
		Use:   "node",
		Short: "manage a opensvc cluster node",
	}

	nodePrintCmd = &cobra.Command{
		Use:     "print",
		Short:   "print node",
		Aliases: []string{"prin", "pri", "pr"},
	}
	nodePushCmd = &cobra.Command{
		Use:   "push",
		Short: "data pushing commands",
	}
	nodeScanCmd = &cobra.Command{
		Use:   "scan",
		Short: "scan node",
	}
	nodeEditCmd = &cobra.Command{
		Use:     "edit",
		Short:   "edition command group",
		Aliases: []string{"edi", "ed"},
	}

	cmdNodeChecks            commands.CmdNodeChecks
	cmdNodeDoc               commands.NodeDoc
	cmdNodeDelete            commands.NodeDelete
	cmdNodeDrivers           commands.NodeDrivers
	cmdNodeEditConfig        commands.NodeEditConfig
	cmdNodeLs                commands.NodeLs
	cmdNodeGet               commands.NodeGet
	cmdNodeEval              commands.NodeEval
	cmdNodePrintCapabilities commands.NodePrintCapabilities
	cmdNodePrintConfig       commands.NodePrintConfig
	cmdNodePushAsset         commands.NodePushAsset
	cmdNodeScanCapabilities  commands.NodeScanCapabilities
	cmdNodeSet               commands.NodeSet
	cmdNodeUnset             commands.NodeUnset
)

func init() {
	root.AddCommand(nodeCmd)
	nodeCmd.AddCommand(nodeEditCmd)
	nodeCmd.AddCommand(nodePrintCmd)
	nodeCmd.AddCommand(nodePushCmd)
	nodeCmd.AddCommand(nodeScanCmd)

	cmdNodeChecks.Init(nodeCmd)
	cmdNodeDoc.Init(nodeCmd)
	cmdNodeDelete.Init(nodeCmd)
	cmdNodeDrivers.Init(nodeCmd)
	cmdNodeEditConfig.Init(nodeEditCmd)
	cmdNodeLs.Init(nodeCmd)
	cmdNodeGet.Init(nodeCmd)
	cmdNodeEval.Init(nodeCmd)
	cmdNodePrintCapabilities.Init(nodePrintCmd)
	cmdNodePrintConfig.Init(nodePrintCmd)
	cmdNodePushAsset.Init(nodePushCmd)
	cmdNodePushAsset.InitAlt(nodeCmd)
	cmdNodeScanCapabilities.Init(nodeScanCmd)
	cmdNodeSet.Init(nodeCmd)
	cmdNodeUnset.Init(nodeCmd)
}
