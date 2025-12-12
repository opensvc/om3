package commoncmd

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdDaemonLog struct {
		CmdDaemonSubAction
		Level string
	}
)

func NewCmdDaemonLog() *cobra.Command {
	options := CmdDaemonLog{}
	cmd := &cobra.Command{
		Use:   "log",
		Short: fmt.Sprintf("configure the daemon logger"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	FlagDaemonLogLevel(flags, &options.Level)
	return cmd
}

func (t *CmdDaemonLog) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonLogControl(ctx, nodename, api.PostDaemonLogControlJSONRequestBody{Level: t.Level})
	}
	return t.CmdDaemonSubAction.Run(fn)
}
