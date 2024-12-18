package oxcmd

import (
	"fmt"

	"github.com/opensvc/om3/core/client"
)

type (
	CmdNetworkSetup struct {
		OptsGlobal
	}
)

func (t *CmdNetworkSetup) Run() error {
	var (
		c   *client.T
		err error
	)
	if c, err = client.New(); err != nil {
		return err
	}
	return fmt.Errorf("todo %v", c)
}
