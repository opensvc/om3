package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcDelete commands.CmdObjectDelete
)

func init() {
	svcDelete.Init("svc", svcCmd, &selectorFlag)
}
