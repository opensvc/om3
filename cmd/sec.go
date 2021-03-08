package cmd

import (
	"github.com/spf13/cobra"
)

var secSelectorFlag string

var secCmd = &cobra.Command{
	Use:   "sec",
	Short: "Manage secrets",
	Long: ` A secret is an encypted key-value store.

Values can be binary or text.

A key can be installed as a file in a Vol, then exposed to apps
and containers.

A key can be exposed as a environment variable for apps and
containers.

A signal can be sent to consumer processes upon exposed key value
changes.
	
The key names can include the '/' character, interpreted as a path separator
when installing the key in a volume.
`,
}

func init() {
	rootCmd.AddCommand(secCmd)
	secCmd.PersistentFlags().StringVarP(&secSelectorFlag, "selector", "s", "", "The name of the object to select")
}
