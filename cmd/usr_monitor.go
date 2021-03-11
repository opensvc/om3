package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	usrMonitor commands.CmdObjectMonitor
)

func init() {
	usrMonitor.Init("usr", usrCmd, &selectorFlag)
}
