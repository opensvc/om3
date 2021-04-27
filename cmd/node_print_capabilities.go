package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

func init() {
	var command commands.NodePrintCapabilities
	command.Init(nodePrint)
}
