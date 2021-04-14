package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	volDelete commands.CmdObjectDelete
)

func init() {
	volDelete.Init("vol", volCmd, &selectorFlag)
}
