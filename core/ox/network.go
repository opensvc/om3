package ox

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

var (
	cmdNetwork  = commoncmd.NewCmdNetwork()
	cmdNetworkIP = commoncmd.NewCmdNetworkIP()
)

func init() {
	root.AddCommand(
		cmdNetwork,
	)
	cmdNetwork.AddCommand(
		cmdNetworkIP,
		newCmdNetworkList(),
		newCmdNetworkSetup(),
	)
	cmdNetworkIP.AddCommand(
		newCmdNetworkIPList(),
	)
}
