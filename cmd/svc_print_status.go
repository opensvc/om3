package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcPrintStatus commands.CmdObjectPrintStatus
)

func init() {
	svcPrintStatus.Init("svc", svcPrintCmd, &selectorFlag)
}
