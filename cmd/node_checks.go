package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	nodeChecks commands.CmdNodeChecks
)

func init() {
	nodeChecks.Init(nodeCmd)
}
