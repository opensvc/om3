package commands

import (
	"fmt"
	"time"
)

type (
	CmdDaemonAuth struct {
		OptsGlobal
		Roles    []string
		Duration time.Duration
	}
)

func (t *CmdDaemonAuth) Run() error {
	return fmt.Errorf("Not yet implemented\n")
}

