package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	secLs commands.CmdObjectLs
)

func init() {
	secLs.Init("sec", secCmd, &selectorFlag)
}
