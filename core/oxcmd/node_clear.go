package oxcmd

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/core/client"
)

type CmdNodeClear struct {
	OptsGlobal
}

func (t *CmdNodeClear) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	if resp, err := c.PostNodeClear(context.Background()); err != nil {
		return err
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpcted post node clear status code %s", resp.Status)
	}
	return nil
}
