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
		Aliases: []string{"compli", "comp", "com", "co"},
	}
}

func NewCmdObjectConfig(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "config",
		Short:   "show, alter object configuration",
	}
}

func NewCmdObjectInstance(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "instance",
		Short:   "query, action object instances",
		Aliases: []string{"inst", "in"},
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
		Hidden:  true,
		Aliases: []string{"syn", "sy"},
	}
}

func NewCmdObjectInstanceSync(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "sync",
		Short:   "replicate object instance data",
		Aliases: []string{"syn", "sy"},
	}
}

func NewCmdObjectResource(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "resource",
		Short:   "query object instance resources",
		Aliases: []string{"res"},
	}
}
