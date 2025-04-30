package commoncmd

import "github.com/spf13/cobra"

func NewCmdAnyConfigUpdate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "update configuration",
		Long: `Apply a batch of configuration changes as a single transaction.

Changes are applied to a buffer in a this order:
1/ section deletes given by --delete=<section>
2/ keyword unsets given by --unset=<section>.<option>
3/ keyword sets given by --set=<section>.<option><op><value>

Then validate the new configuration.
Finally commit if no error was found.

Valid operators are:
* = set exact value
* |= append value if not already in list
* += append value even if already in list
* -= remove value from the list

Validate the new configuration and commit.`,
	}
	return cmd
}
