package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	usrLs commands.CmdObjectLs
)

func init() {
	usrLs.Init("usr", usrCmd, &selectorFlag)
}
