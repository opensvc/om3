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

	nodeComplianceCmd = &cobra.Command{
		Use:     "compliance",
		Short:   "node system configuration queries, checks and fixes",
		Aliases: []string{"compli", "comp", "com", "co"},
	}
	nodeComplianceShowCmd = &cobra.Command{
		Use:     "show",
		Short:   "node system configuration framework queries",
		Aliases: []string{"compli", "comp", "com", "co"},
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

	cmdNodeChecks                commands.CmdNodeChecks
	cmdNodeComplianceShowRuleset commands.CmdNodeComplianceShowRuleset
	cmdNodeDoc                   commands.NodeDoc
	cmdNodeDelete                commands.NodeDelete
	cmdNodeDrivers               commands.NodeDrivers
	cmdNodeEditConfig            commands.NodeEditConfig
	cmdNodeLs                    commands.NodeLs
	cmdNodeGet                   commands.NodeGet
	cmdNodeEval                  commands.NodeEval
	cmdNodePrintCapabilities     commands.NodePrintCapabilities
	cmdNodePrintConfig           commands.NodePrintConfig
	cmdNodePushAsset             commands.NodePushAsset
	cmdNodePushDisks             commands.NodePushDisks
	cmdNodePushPatch             commands.NodePushPatch
	cmdNodePushPkg               commands.NodePushPkg
	cmdNodeRegister              commands.CmdNodeRegister
	cmdNodeScanCapabilities      commands.NodeScanCapabilities
	cmdNodeSet                   commands.NodeSet
	cmdNodeSysreport             commands.CmdNodeSysreport
	cmdNodeUnset                 commands.NodeUnset
)

func init() {
	root.AddCommand(nodeCmd)
	nodeCmd.AddCommand(nodeComplianceCmd)
	nodeComplianceCmd.AddCommand(nodeComplianceShowCmd)
	nodeCmd.AddCommand(nodeEditCmd)
	nodeCmd.AddCommand(nodePrintCmd)
	nodeCmd.AddCommand(nodePushCmd)
	nodeCmd.AddCommand(nodeScanCmd)

	cmdNodeChecks.Init(nodeCmd)
	cmdNodeComplianceShowRuleset.Init(nodeComplianceShowCmd)
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
	cmdNodePushDisks.Init(nodePushCmd)
	cmdNodePushDisks.InitAlt(nodeCmd)
	cmdNodePushPatch.Init(nodePushCmd)
	cmdNodePushPatch.InitAlt(nodeCmd)
	cmdNodePushPkg.Init(nodePushCmd)
	cmdNodePushPkg.InitAlt(nodeCmd)
	cmdNodeRegister.Init(nodeCmd)
	cmdNodeScanCapabilities.Init(nodeScanCmd)
	cmdNodeSet.Init(nodeCmd)
	cmdNodeSysreport.Init(nodeCmd)
	cmdNodeUnset.Init(nodeCmd)
}
