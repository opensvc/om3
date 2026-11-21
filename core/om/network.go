package om

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

var (
	cmdNetwork   = commoncmd.NewCmdNetwork()
	cmdNetworkIP = commoncmd.NewCmdNetworkIP()
)

func init() {
	root.AddCommand(
		cmdNetwork,
	)
	cmdNetworkNode := commoncmd.NewCmdNetworkNode()
	cmdNetworkNode.AddCommand(commoncmd.NewCmdNetworkNodeList())
	cmdNetworkPeer := commoncmd.NewCmdNetworkPeer()
	cmdNetworkPeer.AddCommand(commoncmd.NewCmdNetworkPeerList())
	cmdNetwork.AddCommand(
		cmdNetworkIP,
		cmdNetworkNode,
		cmdNetworkPeer,
		newCmdNetworkList(),
		newCmdNetworkSetup(),
	)
	cmdNetworkIP.AddCommand(
		newCmdNetworkIPList(),
	)
}
