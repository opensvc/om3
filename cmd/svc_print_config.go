package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcPrintConfig commands.CmdObjectPrintConfig
)

func init() {
	svcPrintConfig.Init("svc", svcPrintCmd, &selectorFlag)
}
