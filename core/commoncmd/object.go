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
		Short:   "show, alter object configuration",
	}
}

func NewCmdObjectContainer(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "container",
		Short:   "enter, stream logs",
	}
}

func NewCmdObjectIP(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "ip",
		Short:   "query, action ip resources",
		Aliases: []string{"ipaddr", "address"},
	}
}

func NewCmdObjectFS(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "fs",
		Short:   "query, action fs resources",
		Aliases: []string{"filesystem"},
	}
}

func NewCmdObjectVolume(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "volume",
		Short:   "query, action volume resources",
		Aliases: []string{"vol"},
	}
}

func NewCmdObjectDisk(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "disk",
		Short:   "query, action disk resources",
	}
}

func NewCmdObjectShare(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "share",
		Short:   "query, action share resources",
	}
}

func NewCmdObjectApp(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "app",
		Short:   "query, action app resources",
		Aliases: []string{"application"},
	}
}

func NewCmdObjectGroups(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "groups",
		Short:   "query, action groups resources",
		Aliases: []string{"grp"},
	}
}

func NewCmdObjectTask(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "task",
		Short:   "list, run tasks",
	}
}

func NewCmdObjectInstance(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "instance",
		Short:   "query, action object instances",
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
		GroupID: GroupIDSubsystems,
		Use:     "sync",
		Short:   "replicate data, list sync resources",
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

func NewCmdObjectResource(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "resource",
		Short:   "query, action object instance resources",
		Aliases: []string{"res"},
	}
}

func NewCmdObjectResourceInfo(kind string) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "list, push the key-values reported by resources",
	}
}
