package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcStatus commands.CmdObjectStatus
)

func init() {
	svcStatus.Init("svc", svcCmd, &selectorFlag)
}
