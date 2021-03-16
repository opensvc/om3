package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcPrintStatus commands.CmdObjectPrintStatus
)

func init() {
	svcStatus.Init("svc", svcPrintCmd, &selectorFlag)
}
