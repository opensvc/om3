package cmd

import (
	"github.com/spf13/cobra"
)

var cfgSelectorFlag string

var cfgCmd = &cobra.Command{
	Use:   "cfg",
	Short: "Manage configmaps",
	Long: ` A configmap is an unencrypted key-value store.

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
	rootCmd.AddCommand(cfgCmd)
}
