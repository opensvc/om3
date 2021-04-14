package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	usrDelete commands.CmdObjectDelete
)

func init() {
	usrDelete.Init("usr", usrCmd, &selectorFlag)
}
