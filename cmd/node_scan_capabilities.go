package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

func init() {
	var command commands.NodeScanCapabilities
	command.Init(nodeScanCmd)
}
