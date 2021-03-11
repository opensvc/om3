package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	cfgGet commands.CmdObjectGet
)

func init() {
	cfgGet.Init("cfg", cfgCmd, &selectorFlag)
}
