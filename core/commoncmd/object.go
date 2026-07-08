package commoncmd

import "github.com/spf13/cobra"

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
	return &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "container",
		Short:   "list, start, stop, enter, logs, ...",
	}
}

func NewCmdObjectIP(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "ip",
		Short:   "list, start, stop, ...",
		Aliases: []string{"ipaddr", "address"},
	}
}

func NewCmdObjectFS(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "fs",
		Short:   "list, start, stop, ...",
		Aliases: []string{"filesystem"},
	}
}

func NewCmdObjectVolume(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "volume",
		Short:   "list, start, stop, ...",
		Aliases: []string{"vol"},
	}
}

func NewCmdObjectDisk(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "disk",
		Short:   "list, start, stop, ...",
	}
}

func NewCmdObjectShare(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "share",
		Short:   "list, start, stop, ...",
	}
}

func NewCmdObjectApp(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "app",
		Short:   "list, start, stop, ...",
		Aliases: []string{"application"},
	}
}

func NewCmdObjectTask(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "task",
		Short:   "list, run, ...",
	}
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

func NewCmdObjectSync(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDResourceGroups,
		Use:     "sync",
		Short:   "list, update, ...",
		Long:    "Replicate instance data, execute sync actions, list sync resources.",
		Aliases: []string{"syn", "sy"},
	}
}

func NewCmdObjectInstanceSync(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "sync",
		Short:   "replicate object instance data",
	}
}

func NewCmdObjectResourceSync(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "sync",
		Short:   "replicate object instance data",
	}
	cmd.AddGroup(
		NewGroupSubsystems(),
	)
	return cmd
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
		NewGroupSubsystems(),
	)
	return cmd
}

func NewCmdObjectResourceInfo(kind string) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "list, push the key-values reported by resources",
	}
}
