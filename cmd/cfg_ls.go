package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	cfgLs commands.CmdObjectLs
)

func init() {
	cfgLs.Init("cfg", cfgCmd, &selectorFlag)
}
