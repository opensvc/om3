package cmd

import (
	"github.com/spf13/cobra"
)

var (
	cmdNode = &cobra.Command{
		Use:   "node",
		Short: "manage a opensvc cluster node",
	}

	cmdNodeCompliance = &cobra.Command{
		Use:     "compliance",
		Short:   "node configuration manager commands",
		Aliases: []string{"compli", "comp", "com", "co"},
	}
	cmdNodeComplianceAttach = &cobra.Command{
		Use:     "attach",
		Short:   "attach modulesets and rulesets to the node",
		Aliases: []string{"attac", "atta", "att", "at"},
	}
	cmdNodeComplianceDetach = &cobra.Command{
		Use:     "detach",
		Short:   "detach modulesets and rulesets from the node",
		Aliases: []string{"detac", "deta", "det", "de"},
	}
	cmdNodeComplianceList = &cobra.Command{
		Use:     "list",
		Short:   "list modules, modulesets and rulesets available",
		Aliases: []string{"lis", "li", "ls", "l"},
	}
	cmdNodeComplianceShow = &cobra.Command{
		Use:     "show",
		Short:   "show modules, modulesets, rulesets, modules, attachments",
		Aliases: []string{"sho", "sh", "s"},
	}
	cmdNodePrint = &cobra.Command{
		Use:     "print",
		Short:   "print node discover information",
		Aliases: []string{"prin", "pri", "pr"},
	}
	cmdNodePush = &cobra.Command{
		Use:   "push",
		Short: "push node discover information to the collector",
	}
	cmdNodeScan = &cobra.Command{
		Use:   "scan",
		Short: "node discover",
	}
	cmdNodeValidate = &cobra.Command{
		Use:     "validate",
		Short:   "validate the node config syntax",
		Aliases: []string{"validat", "valida", "valid", "val"},
	}

	cmdNodeEdit = newCmdNodeEdit()
)

func init() {
	root.AddCommand(cmdNode)
	cmdNode.AddCommand(cmdNodeCompliance)
	cmdNodeCompliance.AddCommand(
		cmdNodeComplianceAttach,
		cmdNodeComplianceDetach,
		cmdNodeComplianceShow,
		cmdNodeComplianceList,
		newCmdNodeComplianceEnv(),
		newCmdNodeComplianceAuto(),
		newCmdNodeComplianceCheck(),
		newCmdNodeComplianceFix(),
		newCmdNodeComplianceFixable(),
	)
	cmdNodeComplianceAttach.AddCommand(
		newCmdNodeComplianceAttachModuleset(),
		newCmdNodeComplianceAttachRuleset(),
	)
	cmdNodeComplianceDetach.AddCommand(
		newCmdNodeComplianceDetachModuleset(),
		newCmdNodeComplianceDetachRuleset(),
	)
	cmdNodeComplianceShow.AddCommand(
		newCmdNodeComplianceShowRuleset(),
		newCmdNodeComplianceShowModuleset(),
	)
	cmdNodeComplianceList.AddCommand(
		newCmdNodeComplianceListModules(),
		newCmdNodeComplianceListModuleset(),
		newCmdNodeComplianceListRuleset(),
	)
	cmdNodeEdit.AddCommand(
		newCmdNodeEditConfig(),
	)
	cmdNode.AddCommand(
		cmdNodeEdit,
		cmdNodePrint,
		cmdNodePush,
		cmdNodeScan,
		cmdNodeValidate,
		newCmdNodeAbort(),
		newCmdNodeChecks(),
		newCmdNodeClear(),
		newCmdNodeDoc(),
		newCmdNodeDelete(),
		newCmdNodeDrain(),
		newCmdNodeDrivers(),
		newCmdNodeLogs(),
		newCmdNodeLs(),
		newCmdNodePRKey(),
		newCmdNodeFreeze(),
		newCmdNodeGet(),
		newCmdNodeEvents(),
		newCmdNodeEval(),
		newCmdNodeRegister(),
		newCmdNodeSet(),
		newCmdNodeSysreport(),
		newCmdNodeUnfreeze(),
		newCmdNodeUnset(),
	)
	cmdNodePrint.AddCommand(
		newCmdNodePrintCapabilities(),
		newCmdNodePrintConfig(),
		newCmdNodePrintSchedule(),
	)
	cmdNodePush.AddCommand(
		newCmdNodePushAsset(),
		newCmdNodePushDisks(),
		newCmdNodePushPatch(),
		newCmdNodePushPkg(),
	)
	cmdNodeScan.AddCommand(
		newCmdNodeScanCapabilities(),
	)
	cmdNodeValidate.AddCommand(
		newCmdNodeValidateConfig(),
	)

}
