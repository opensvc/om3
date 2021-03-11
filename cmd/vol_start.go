package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	volStart commands.CmdObjectStart
)

func init() {
	volStart.Init("vol", volCmd, &selectorFlag)
}
