package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcEditConfig commands.CmdObjectEditConfig
)

func init() {
	svcEditConfig.Init("svc", svcEditCmd, &selectorFlag)
}
