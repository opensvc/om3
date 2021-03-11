package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	cfgMonitor commands.CmdObjectMonitor
)

func init() {
	cfgMonitor.Init("cfg", cfgCmd, &selectorFlag)
}
