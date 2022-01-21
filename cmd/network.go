package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/commands"
)

var (
	networkCmd = &cobra.Command{
		Use:     "network",
		Short:   "Manage backend networks",
		Aliases: []string{"net"},
		Long:    ` A backend network provides ip addresses to svc objects via ip.cni resources. These addresses are automatically allocated, accessible from all cluster nodes, and resolved by the cluster dns.`,
	}
)

func init() {
	var (
		cmdNetworkLs     commands.NetworkLs
		cmdNetworkSetup  commands.NetworkSetup
		cmdNetworkStatus commands.NetworkStatus
	)
	root.AddCommand(networkCmd)

	cmdNetworkLs.Init(networkCmd)
	cmdNetworkSetup.Init(networkCmd)
	cmdNetworkStatus.Init(networkCmd)
}
