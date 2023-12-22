package cmd

import (
	"github.com/spf13/cobra"
)

var (
	cmdDNS = &cobra.Command{
		Use:   "dns",
		Short: "dns subsystem commands",
	}
)

func init() {
	root.AddCommand(
		cmdDNS,
	)
	cmdDNS.AddCommand(
		newCmdDNSDump(),
	)
}
