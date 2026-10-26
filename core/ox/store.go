package ox

import (
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/commoncmd"
	commands "github.com/opensvc/om3/v3/core/oxcmd"
)

func init() {
	root.AddCommand(newCmdStore())
}

func newCmdStore() *cobra.Command {
	cmd := commoncmd.NewCmdStore()
	cmd.AddGroup(commoncmd.NewGroupQuery())
	snapshot := commoncmd.NewCmdStoreSnapshot()
	snapshot.AddCommand(
		newCmdStoreSnapshotList(),
		newCmdStoreSnapshotRm(),
	)
	cmd.AddCommand(
		newCmdStoreList(),
		snapshot,
	)
	return cmd
}

func newCmdStoreList() *cobra.Command {
	var options commands.CmdStoreList
	cmd := commoncmd.NewCmdStoreList()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return options.Run()
	}
	addFlagsGlobal(cmd.Flags(), &options.OptsGlobal)
	return cmd
}

func newCmdStoreSnapshotList() *cobra.Command {
	var options commands.CmdStoreSnapshotList
	cmd := commoncmd.NewCmdStoreSnapshotList()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return options.Run()
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	flags.StringVar(&options.Store, "store", "", "the store to list, every store when not set")
	flags.StringVar(&options.Path, "path", "", "the object whose snapshots to list, by the path the store last recorded for it, every object when not set")
	flags.StringVar(&options.Cluster, "cluster", "", "the cluster whose snapshots to list, by name or id, every cluster when not set")
	return cmd
}

func newCmdStoreSnapshotRm() *cobra.Command {
	var options commands.CmdStoreSnapshotRm
	cmd := commoncmd.NewCmdStoreSnapshotRm()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return options.Run()
	}
	flags := cmd.Flags()
	addFlagsGlobal(flags, &options.OptsGlobal)
	flags.StringVar(&options.Store, "store", "", "the store holding the snapshot: a store name, or from <sec> store#<name>")
	flags.StringVar(&options.Snapshot, "snapshot", "", "the snapshot to remove, by id or a prefix of it")
	_ = cmd.MarkFlagRequired("store")
	_ = cmd.MarkFlagRequired("snapshot")
	return cmd
}
