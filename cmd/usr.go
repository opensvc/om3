package cmd

import (
	"github.com/spf13/cobra"
)

var usrSelectorFlag string

var usrCmd = &cobra.Command{
	Use:   "usr",
	Short: "Manage users",
	Long: `A user stores the grants and credentials of user of the agent API.

User objects are not necessary with OpenID authentication, as the
grants are embedded in the trusted bearer tokens.
`,
}

func init() {
	rootCmd.AddCommand(usrCmd)
	usrCmd.PersistentFlags().StringVarP(&usrSelectorFlag, "selector", "s", "", "The name of the object to select")
}
