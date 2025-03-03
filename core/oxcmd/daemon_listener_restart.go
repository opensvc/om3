package oxcmd

import (
	"context"
	"net/http"

	"github.com/opensvc/om3/core/client"
)

type (
	CmdDaemonListenerRestart struct {
		CmdDaemonSubAction
	}
)

func (t *CmdDaemonListenerRestart) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonListenerRestart(ctx, nodename, t.Name)
	}
	return t.CmdDaemonSubAction.Run(fn)
}
