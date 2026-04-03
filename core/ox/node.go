package ox

import (
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/commoncmd"
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
	cmdNodeSCSI = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "scsi",
		Short:   "scsi commands",
	}
	cmdNodeCollectorTag = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "tag",
		Short:   "collector tags commands",
	}
	cmdNodeCompliance = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "compliance",
		Short:   "node configuration manager commands",
		Aliases: []string{"comp"},
	}
	cmdNodeRelay = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "relay",
		Short:   "relay commands",
	}
	cmdNodeSchedule = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "schedule",
		Short:   "scheduler commands",
		Aliases: []string{"sched"},
	}
	cmdNodeSSH = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "ssh",
		Short:   "ssh commands",
	}
	cmdNodeConfig = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "config",
		Short:   "node configuration commands",
	}

	// backward compat

	cmdNodePrint = &cobra.Command{
		Use:     "print",
		Short:   "print node discover information",
		Aliases: []string{"prin", "pri", "pr"},
	}
	cmdNodePush = &cobra.Command{
		Use:   "push",
		Short: "push node discover information to the collector",
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

	cmdNodeSystem    = newCmdNodeSystem()
	cmdNodeSystemSAN = newCmdNodeSystemSAN()
	cmdNodeEdit      = newCmdNodeEdit()
	cmdNodeValidate  = newCmdNodeValidate()
)

func init() {
	root.AddCommand(cmdNode)
	cmdNode.AddCommand(cmdNodeCapabilities)
	cmdNode.AddGroup(
		commoncmd.NewGroupSubsystems(),
	)
	cmdNodeCollector.AddGroup(
		commoncmd.NewGroupSubsystems(),
	)

	cmdNodeCapabilities.AddCommand(
		newCmdNodeCapabilitiesList(),
		newCmdNodeCapabilitiesScan(),
	)
	cmdNode.AddCommand(cmdNodeSCSI)
	cmdNodeSCSI.AddCommand(
		newCmdNodeSCSIScan(),
		newCmdNodeSCSIPRKey(),
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
	cmdNodeSystem.AddCommand(
		cmdNodeSystemSAN,
		newCmdNodeSystemDisk(),
		newCmdNodeSystemGroup(),
		newCmdNodeSystemHardware(),
		newCmdNodeSystemIPAddress(),
		newCmdNodeSystemPackage(),
		newCmdNodeSystemPatch(),
		newCmdNodeSystemProperty(),
		newCmdNodeSystemUser(),
	)

	cmdNodeSystemSAN.AddCommand(
		newCmdNodeSystemSANPathInitiator(),
		newCmdNodeSystemSANPath(),
	)
	cmdNodeConfig.AddCommand(
		commoncmd.NewCmdNodeConfigDoc(),
		newCmdNodeConfigEdit(),
		newCmdNodeConfigEval(),
		newCmdNodeConfigGet(),
		newCmdNodeConfigShow(),
		newCmdNodeConfigUpdate(),
		newCmdNodeConfigValidate(),
	)
	cmdNodeEdit.AddCommand(
		newCmdNodeEditConfig(),
	)
	cmdNode.AddCommand(
		cmdNodeSystem,
		cmdNodeConfig,
		cmdNodeEdit,
		cmdNodePrint,
		cmdNodePush,
		cmdNodeRelay,
		cmdNodeSchedule,
		cmdNodeSSH,
		cmdNodeValidate,
		newCmdNodeAbort(),
		newCmdNodeChecks(),
		newCmdNodeClear(),
		newCmdNodeDrain(),
		newCmdNodeDrivers(),
		newCmdNodeLogs(),
		newCmdNodeList(),
		newCmdNodePing(),
		newCmdNodeFreeze(),
		newCmdNodeGet(),
		newCmdNodeEvents(),
		newCmdNodeEval(),
		newCmdNodeRegister(),
		newCmdNodeSet(),
		newCmdNodeSysreport(),
		newCmdNodeUnfreeze(),
		newCmdNodeUpdate(),
		newCmdNodeUnset(),
	)
	cmdNodePrint.AddCommand(
		newCmdNodePrintConfig(),
		newCmdNodePrintSchedule(),
	)
	cmdNodePush.AddCommand(
		newCmdNodePushAsset(),
		newCmdNodePushDisk(),
		newCmdNodePushPatch(),
		newCmdNodePushPkg(),
	)
	cmdNodeValidate.AddCommand(
		newCmdNodeValidateConfig(),
	)
	cmdNodeRelay.AddCommand(
		newCmdNodeRelayStatus(),
	)
	cmdNodeSchedule.AddCommand(
		newCmdNodeScheduleList(),
	)
	cmdNodeSSH.AddCommand(
		newCmdNodeSSHTrust(),
	)

}
