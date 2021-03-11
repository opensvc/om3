package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	volLs commands.CmdObjectLs
)

func init() {
	volLs.Init("vol", volCmd, &selectorFlag)
}
