package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	secGet commands.CmdObjectGet
)

func init() {
	secGet.Init("sec", secCmd, &selectorFlag)
}
