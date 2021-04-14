package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	secDelete commands.CmdObjectDelete
)

func init() {
	secDelete.Init("sec", secCmd, &selectorFlag)
}
