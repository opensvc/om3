package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	rootCreate commands.CmdObjectCreate
)

func init() {
	rootCreate.Init("*", rootCmd, &selectorFlag)
}
