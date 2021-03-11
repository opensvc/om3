package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	volStop commands.CmdObjectStop
)

func init() {
	volStop.Init("vol", volCmd, &selectorFlag)
}
