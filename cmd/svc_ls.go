package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcLs commands.CmdObjectLs
)

func init() {
	svcLs.Init("svc", svcCmd, &selectorFlag)
}
