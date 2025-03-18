package oxcmd

import (
	"context"
	"net/http"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdDaemonListenerLog struct {
		CmdDaemonSubAction
		Level string
	}
)

func (t *CmdDaemonListenerLog) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonListenerLogControl(ctx, nodename, t.Name, api.PostDaemonListenerLogControlJSONRequestBody{Level: t.Level})
	}
	return t.CmdDaemonSubAction.Run(fn)
}
