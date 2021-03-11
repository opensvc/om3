package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcUnfreeze commands.CmdObjectUnfreeze
)

func init() {
	svcUnfreeze.Init("svc", svcCmd, &selectorFlag)
}
