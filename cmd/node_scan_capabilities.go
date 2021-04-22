package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	nodeScanCapabilities commands.CmdNodeScanCapabilities
)

func init() {
	nodeScanCapabilities.Init(nodeScanCmd)
}
