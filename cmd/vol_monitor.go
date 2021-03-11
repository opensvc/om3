package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	volMonitor commands.CmdObjectMonitor
)

func init() {
	volMonitor.Init("vol", volCmd, &selectorFlag)
}
