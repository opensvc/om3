package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	cfgDelete commands.CmdObjectDelete
)

func init() {
	cfgDelete.Init("cfg", cfgCmd, &selectorFlag)
}
