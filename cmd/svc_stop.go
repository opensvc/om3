package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcStop commands.CmdObjectStop
)

func init() {
	svcStop.Init("svc", svcCmd, &selectorFlag)
}
