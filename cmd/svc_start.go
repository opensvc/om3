package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcStart commands.CmdObjectStart
)

func init() {
	svcStart.Init("svc", svcCmd, &selectorFlag)
}
