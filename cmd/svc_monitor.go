package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcMonitor commands.CmdObjectMonitor
)

func init() {
	svcMonitor.Init("svc", svcCmd, &selectorFlag)
}
