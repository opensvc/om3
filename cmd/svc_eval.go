package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcEval commands.CmdObjectEval
)

func init() {
	svcEval.Init("svc", svcCmd, &selectorFlag)
}
