package commoncmd

import "github.com/spf13/cobra"

func NewCmdObjectCollector(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "collector",
		Short:   "collector data management commands",
		Aliases: []string{"coll"},
	}
	return cmd
}

func NewCmdObjectCompliance(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "compliance",
		Short:   "node configuration expectations analysis and application",
		Aliases: []string{"compli", "comp", "com", "co"},
	}
}

func NewCmdObjectConfig(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "config",
		Short:   "object configuration commands",
	}
}

func NewCmdObjectInstance(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "instance",
		Short:   "config, status, monitor, list",
		Aliases: []string{"inst", "in"},
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
		Short:   "data key commands",
	}
}

func NewCmdObjectSSH(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "ssh",
		Short:   "ssh command group",
	}
}

func NewCmdObjectSync(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "sync",
		Short:   "data synchronization command group",
		Aliases: []string{"syn", "sy"},
	}
}

func NewCmdObjectResource(kind string) *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "resource",
		Short:   "config, status, monitor, list",
		Aliases: []string{"res"},
	}
}
