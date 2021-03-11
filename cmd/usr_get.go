package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	usrGet commands.CmdObjectGet
)

func init() {
	usrGet.Init("usr", usrCmd, &selectorFlag)
}
