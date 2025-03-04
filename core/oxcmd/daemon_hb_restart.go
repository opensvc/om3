package oxcmd

import (
	"context"
	"net/http"

	"github.com/opensvc/om3/core/client"
)

type (
	CmdDaemonHeartbeatRestart struct {
		CmdDaemonSubAction
	}
)

func (t *CmdDaemonHeartbeatRestart) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonHeartbeatRestart(ctx, nodename, t.Name)
	}
	return t.CmdDaemonSubAction.Run(fn)
}
