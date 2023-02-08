package commands

import (
	"github.com/opensvc/om3/core/client"
)

type CmdNodeClear struct {
	OptsGlobal
}

func (t *CmdNodeClear) Run() error {
	c, err := client.New(
		client.WithURL(t.Server),
	)
	if err != nil {
		return err
	}
	req := c.NewPostNodeClear()
	_, err = req.Do()
	return err
}
