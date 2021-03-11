package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	volGet commands.CmdObjectGet
)

func init() {
	volGet.Init("vol", volCmd, &selectorFlag)
}
