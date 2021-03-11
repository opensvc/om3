package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcFreeze commands.CmdObjectFreeze
)

func init() {
	svcFreeze.Init("svc", svcCmd, &selectorFlag)
}
