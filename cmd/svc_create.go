package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcCreate commands.CmdObjectCreate
)

func init() {
	svcCreate.Init("svc", svcCmd, &selectorFlag)
}
