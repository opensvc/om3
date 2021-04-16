package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcUnset commands.CmdObjectUnset
)

func init() {
	svcUnset.Init("svc", svcCmd, &selectorFlag)
}
