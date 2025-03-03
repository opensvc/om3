package omcmd

import (
	"context"
	"net/http"

	"github.com/opensvc/om3/core/client"
)

type (
	CmdDaemonHeartbeatStart struct {
		CmdDaemonSubAction
	}
)

func (t *CmdDaemonHeartbeatStart) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonHeartbeatStart(ctx, nodename, t.Name)
	}
	return t.CmdDaemonSubAction.Run(fn)
}
