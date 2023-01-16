package commands

import (
	"github.com/pkg/errors"
)

type (
	CmdDaemonLeave struct{}
)

func (t *CmdDaemonLeave) Run() error {
	return errors.Errorf("TODO")
}
