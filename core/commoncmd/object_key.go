package commoncmd

import (
	"github.com/spf13/cobra"
)

func NewCmdObjectKeyAdd(kind string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [NAME]",
		Short: "add new key",
		Args:  cobra.MaximumNArgs(1),
	}
	CmdWithArg(cmd, "NAME  The key name.")
	return cmd
}

func NewCmdObjectKeyChange(kind string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "change [NAME]",
		Short: "change key value",
		Args:  cobra.MaximumNArgs(1),
	}
	CmdWithArg(cmd, "NAME  The key name.")
	return cmd
}

func NewCmdObjectKeyDecode(kind string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decode [NAME]",
		Short: "decode key value",
		Args:  cobra.MaximumNArgs(1),
	}
	CmdWithArg(cmd, "NAME  The key name.")
	return cmd
}

func NewCmdObjectKeyEdit(kind string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "edit [NAME]",
		Short:   "edit key value",
		Aliases: []string{"ed"},
		Args:    cobra.MaximumNArgs(1),
	}
	CmdWithArg(cmd, "NAME  The key name.")
	return cmd
}

func NewCmdObjectKeyInstall(kind string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [NAME]",
		Short: "install keys as files in volumes",
		Long:  "Keys of sec and cfg can be projected to volumes via the configs and secrets keywords of volume resources. When a key value changes all projections are automatically refreshed. This command triggers manually the same operations. If no key name is given, all keys in the datastore will be reinstalled.",
		Args:  cobra.MaximumNArgs(1),
	}
	CmdWithArg(cmd, "NAME  The key name.")
	return cmd
}

func NewCmdObjectKeyList(kind string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list [PATTERN]",
		Short:   "list the keys",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
	}
	CmdWithArg(cmd, "PATTERN  A fnmatch key name filter.")
	return cmd
}

func NewCmdObjectKeyRemove(kind string) *cobra.Command {
	cmd := &cobra.Command{
		Aliases: []string{"rm"},
		Use:     "remove [NAME]...",
		Short:   "remove key",
	}
	CmdWithArg(cmd, "NAME...  The key names to remove.")
	return cmd
}

func NewCmdObjectKeyRename(kind string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rename [NAME] [TO]",
		Short:   "rename key",
		Aliases: []string{"mv"},
		Args:    cobra.ExactArgs(2),
	}
	CmdWithArg(cmd, "NAME  The key name.\nTO    The new key name.")
	return cmd
}
