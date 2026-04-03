package om

import (
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/omcmd"
	"github.com/opensvc/om3/v3/util/version"
)

var (
	cmdNode             = commoncmd.NewCmdNode()
	cmdNodeCapabilities = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "capabilities",
		Short:   "scan and list what the node is capable of",
		Aliases: []string{"capa", "caps", "cap"},
	}
	cmdNodeCollector = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "collector",
		Short:   "node collector data management commands",
		Aliases: []string{"coll"},
	}
	cmdNodeCollectorTag = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "tag",
		Short:   "collector tags management commands",
	}
	cmdNodeCompliance = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "compliance",
		Short:   "node configuration manager commands",
		Aliases: []string{"comp"},
	}
	cmdNodeConfig = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "config",
		Short:   "configuration commands",
		Aliases: []string{"conf", "c", "cf", "cfg"},
	}
	cmdNodeSCSI = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "scsi",
		Short:   "scsi commands",
	}
	cmdNodeRelay = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "relay",
		Short:   "relay commands",
	}
	cmdNodeScan = &cobra.Command{
		Use:    "scan",
		Hidden: true,
	}
	cmdNodeSchedule = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "schedule",
		Short:   "scheduler commands",
	}
	cmdNodeSSH = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "ssh",
		Short:   "ssh commands",
	}

	// Backward compat

	cmdNodeEdit     = newCmdNodeEdit()
	cmdNodeValidate = newCmdNodeValidate()

	cmdNodeUpdateSSH = &cobra.Command{
		Use:    "ssh",
		Hidden: true,
	}
	cmdNodePrint = &cobra.Command{
		Use:     "print",
		Hidden:  true,
		Aliases: []string{"prin", "pri", "pr"},
	}
	cmdNodePush = &cobra.Command{
		Use:   "push",
		Short: "push node discover information to the collector",
	}
	cmdNodeUpdate = &cobra.Command{
		Use:    "update",
		Hidden: true,
	}
	cmdNodeComplianceAttach = &cobra.Command{
		Use:     "attach",
		Short:   "attach modulesets and rulesets to the node",
		Aliases: []string{"atta", "att", "at"},
	}
	cmdNodeComplianceDetach = &cobra.Command{
		Use:     "detach",
		Short:   "detach modulesets and rulesets from the node",
		Aliases: []string{"deta", "det", "de"},
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
)

// getCmdNodeWithVersion returns cmdNode with --version (for backward compatibility with b2.1)
func getCmdNodeWithVersion() *cobra.Command {
	cmdNode.Version = version.Version()
	cmdNode.SetVersionTemplate(`{{printf "om node version %s\n" .Version}}`)

	// hide --version flag from help
	flags := cmdNode.Flags()
	var showVersion bool
	flags.BoolVar(&showVersion, "version", false, "show version")
	_ = flags.MarkHidden("version")

	return cmdNode
}

func init() {
	// Add backward compatibility for --version flag
	cmdNode = getCmdNodeWithVersion()
	cmdNode.AddGroup(
		//		commoncmd.NewGroupOrchestratedActions(),
		//		commoncmd.NewGroupQuery(),
		commoncmd.NewGroupSubsystems(),
	)
	cmdNodeCollector.AddGroup(
		commoncmd.NewGroupSubsystems(),
	)

	root.AddCommand(cmdNode)
	cmdNode.AddCommand(cmdNodeCapabilities)
	cmdNodeCapabilities.AddCommand(
		newCmdNodeCapabilitiesList(),
		newCmdNodeCapabilitiesScan(),
	)
	cmdNode.AddCommand(cmdNodeCollector)
	cmdNodeCollector.AddCommand(cmdNodeCollectorTag)
	cmdNodeCollectorTag.AddCommand(
		newCmdNodeCollectorTagAttach(),
		newCmdNodeCollectorTagDetach(),
		newCmdNodeCollectorTagCreate(),
		newCmdNodeCollectorTagList(),
		newCmdNodeCollectorTagShow(),
	)
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
		cmdNodeConfig,
		cmdNodeEdit,
		cmdNodePrint,
		cmdNodePush,
		cmdNodeRelay,
		cmdNodeScan,
		cmdNodeSCSI,
		cmdNodeSchedule,
		cmdNodeSSH,
		cmdNodeUpdate,
		cmdNodeValidate,
		newCmdNodeAbort(),
		newCmdNodeChecks(),
		newCmdNodeClear(),
		newCmdNodeDrain(),
		newCmdNodeDrivers(),
		newCmdNodeLogs(),
		newCmdNodeList(),
		newCmdNodePRKey(),
		newCmdNodePushasset(),
		newCmdNodePushdisk(),
		newCmdNodePushpatch(),
		newCmdNodePushpkg(),
		newCmdNodeFreeze(),
		newCmdNodeGet(),
		newCmdNodeEvents(),
		newCmdNodeEval(),
		newCmdNodeRegister(),
		newCmdNodeScanscsi(),
		newCmdNodeSet(),
		newCmdNodeStonith(),
		newCmdNodeSysreport(),
		newCmdNodeUnfreeze(),
		newCmdNodeUnset(),
	)
	cmdNodeConfig.AddCommand(
		omcmd.NewCmdNodeConfigDoc(),
		newCmdNodeConfigEdit(),
		newCmdNodeConfigEval(),
		newCmdNodeConfigGet(),
		newCmdNodeConfigShow(),
		newCmdNodeConfigUpdate(),
		newCmdNodeConfigValidate(),
	)
	cmdNodePrint.AddCommand(
		newCmdNodePrintCapabilities(),
		newCmdNodePrintConfig(),
		newCmdNodePrintSchedule(),
	)
	cmdNodePush.AddCommand(
		newCmdNodePushAsset(),
		newCmdNodePushDisk(),
		newCmdNodePushPatch(),
		newCmdNodePushPkg(),
	)
	cmdNodeRelay.AddCommand(
		newCmdNodeRelayStatus(),
	)
	cmdNodeScan.AddCommand(
		newCmdNodeScanCapabilities(),
	)
	cmdNodeSCSI.AddCommand(
		newCmdNodeSCSIScan(),
		newCmdNodeSCSIPRKey(),
	)
	cmdNodeSchedule.AddCommand(
		newCmdNodeScheduleList(),
	)
	cmdNodeSSH.AddCommand(
		newCmdNodeSSHTrust(),
	)
	cmdNodeUpdateSSH.AddCommand(
		newCmdNodeUpdateSSHKeys(),
	)
	cmdNodeUpdate.AddCommand(
		cmdNodeUpdateSSH,
	)
	cmdNodeValidate.AddCommand(
		newCmdNodeValidateConfig(),
	)

}
