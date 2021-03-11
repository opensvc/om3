package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	secMonitor commands.CmdObjectMonitor
)

func init() {
	secMonitor.Init("sec", secCmd, &selectorFlag)
}
