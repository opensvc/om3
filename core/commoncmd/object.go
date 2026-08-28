package commoncmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCmdObjectCollector(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "collector",
		Short:   "query, push collector data",
		Aliases: []string{"coll"},
	}
	return cmd
}

func NewCmdObjectCompliance(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "compliance",
		Short:   "analyze, enforce node configuration compliance",
		Aliases: []string{"comp"},
	}
}

func NewCmdObjectConfig(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "config",
		Aliases: []string{"conf", "cf", "cfg"},
		Short:   "show, edit, update, ...",
	}
}

func NewCmdObjectContainer(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "container",
		Short:   "list, start, stop, enter, logs, ...",
	}
	cmd.AddGroup(
		NewGroupQuery(),
	)
	return cmd
}

func NewCmdObjectIP(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "ip",
		Short:   "list, start, stop, ...",
		Aliases: []string{"ipaddr", "address"},
	}
	cmd.AddGroup(
		NewGroupQuery(),
	)
	return cmd
}

func NewCmdObjectFS(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "fs",
		Short:   "list, start, stop, ...",
		Aliases: []string{"filesystem"},
	}
	cmd.AddGroup(
		NewGroupQuery(),
	)
	return cmd
}

func NewCmdObjectVolume(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "volume",
		Short:   "list, start, stop, ...",
		Aliases: []string{"vol"},
	}
	cmd.AddGroup(
		NewGroupQuery(),
	)
	return cmd
}

func NewCmdObjectDisk(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "disk",
		Short:   "list, start, stop, ...",
	}
	cmd.AddGroup(
		NewGroupQuery(),
		NewGroupReplication(),
	)
	return cmd
}

func NewCmdObjectShare(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "share",
		Short:   "list, start, stop, ...",
	}
	cmd.AddGroup(
		NewGroupQuery(),
	)
	return cmd
}

func NewCmdObjectApp(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "app",
		Short:   "list, start, stop, ...",
		Aliases: []string{"application"},
	}
	cmd.AddGroup(
		NewGroupQuery(),
	)
	return cmd
}

func NewCmdObjectSync(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "sync",
		Short:   "list, update, ...",
		Long:    "Replicate instance data, execute sync actions, list sync resources.",
	}
	cmd.AddGroup(
		NewGroupQuery(),
		NewGroupReplication(),
	)
	return cmd
}

func NewCmdObjectTask(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "task",
		Short:   "list, run, ...",
	}
	cmd.AddGroup(
		NewGroupQuery(),
	)
	return cmd
}

func NewCmdObjectInstance(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "instance",
		Short:   "list, start, stop, ...",
		Aliases: []string{"inst"},
	}
	cmd.AddGroup(
		NewGroupQuery(),
		NewGroupReplication(),
		NewGroupSubsystems(),
	)
	return cmd
}

func NewCmdObjectInstanceDevice(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "device",
		Short:   "block device commands",
		Aliases: []string{"dev"},
	}
	cmd.AddGroup(
		NewGroupQuery(),
	)
	return cmd
}

func NewCmdObjectKey(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "key",
		Short:   "query, alter datastore keys",
	}
}

func NewCmdObjectSSH(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "ssh",
		Short:   "deploy cluster nodes ssh trust",
	}
}

func NewCmdObjectResource(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "resource",
		Short:   "list, start, stop, ...",
		Aliases: []string{"res"},
	}
	cmd.AddGroup(
		NewGroupQuery(),
		NewGroupReplication(),
		NewGroupSubsystems(),
	)
	return cmd
}

func NewCmdObjectResourceInfo(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "info",
		Short:   "report the key-values reported by resources",
	}
}

func NewCmdObjectPrint(kind string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "print",
		Short: "print information about the object",
	}
	cmd.AddGroup(
		NewGroupQuery(),
	)
	return cmd
}

func NewCmdObjectGroupStart(kind, group string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [PATTERN]...",
		Short: "start resources",
		Long:  "Start resources.",
	}
	CmdWithArg(cmd, "PATTERN  A fnmatch resource index filter.")
	return cmd
}

func NewCmdObjectGroupStop(kind, group string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop [PATTERN]...",
		Short: "stop resources",
		Long:  "Stop resources.",
	}
	CmdWithArg(cmd, "PATTERN  A fnmatch resource index filter.")
	return cmd
}

func NewCmdObjectGroupRestart(kind, group string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart [PATTERN]...",
		Short: "restart resources",
		Long:  "Restart resources.",
	}
	CmdWithArg(cmd, "PATTERN  A fnmatch resource index filter.")
	return cmd
}

func NewCmdObjectGroupProvision(kind, group string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "provision [PATTERN]...",
		Short:   "allocate the system resources of instance resources",
		Long:    "Allocate the system resources required by the selected object instance resources.\n\nFor example, provision a fs.ext3 resource means format the device with the mkfs.ext3 command.",
		Aliases: []string{"prov"},
	}
	CmdWithArg(cmd, "PATTERN  A fnmatch resource index filter.")
	return cmd
}

func NewCmdObjectGroupUnprovision(kind, group string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unprovision [PATTERN]...",
		Short:   "deallocate the system resources of the instance resources",
		Long:    "Deallocate the system resources used by the object instance resources identified by the rid selector.",
		Aliases: []string{"unprov"},
	}
	CmdWithArg(cmd, "PATTERN  A fnmatch resource index filter.")
	return cmd
}

func NewCmdObjectGroupStartStandby(kind, group string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "startstandby [PATTERN]...",
		Short: "start resources in standby mode",
		Long:  "Start resources in standby mode.",
	}
	CmdWithArg(cmd, "PATTERN  A fnmatch resource index filter.")
	return cmd
}

func NewCmdObjectGroupShutdown(kind, group string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shutdown [PATTERN]...",
		Short: "shutdown resources",
		Long:  "Shutdown resources.",
	}
	CmdWithArg(cmd, "PATTERN  A fnmatch resource index filter.")
	return cmd
}

func NewCmdObjectGroupRun(kind, group string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [PATTERN]...",
		Short: "execute resources command",
		Long:  "Execute resources command.",
	}
	CmdWithArg(cmd, "PATTERN  A fnmatch resource index filter.")
	return cmd
}

func NewCmdObjectGroupList(kind, group string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list [PATTERN]...",
		Aliases: []string{"ls"},
		Short:   fmt.Sprintf("list %s resources", group),
		Long:    fmt.Sprintf("List %s resources.", group),
		GroupID: GroupIDQuery,
		Example: fmt.Sprintf(`  # list all %s resources in system/svc/dns
  om system/svc/dns %s ls

  # list all %s resources with index ending with '1' or '2'
  om svc %s ls '*1' '*2'`, group, group, group, group),
	}
	CmdWithArg(cmd, "PATTERN  A fnmatch resource index filter.")
	return cmd
}
