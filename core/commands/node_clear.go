package commands

import (
	"context"
	"net/http"

	"github.com/pkg/errors"

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
	if resp, err := c.PostNodeClear(context.Background()); err != nil {
		return err
	} else if resp.StatusCode != http.StatusOK {
		return errors.Errorf("unexpcted post node clear status code %s", resp.Status)
	}
	return nil
}
