package cmd

import (
	"opensvc.com/opensvc/core/commands"
)

var (
	svcGet commands.CmdObjectGet
)

func init() {
	svcGet.Init("svc", svcCmd, &selectorFlag)
}
