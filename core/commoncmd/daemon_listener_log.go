package commoncmd

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/spf13/cobra"
)

type (
	CmdDaemonListenerLog struct {
		CmdDaemonSubAction
		Name  string
		Level string
	}
)

func NewCmdDaemonListenerLog() *cobra.Command {
	options := CmdDaemonListenerLog{}
	cmd := &cobra.Command{
		Use:   "log",
		Short: fmt.Sprintf("configure a daemon listener logger"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	FlagDaemonListenerName(flags, &options.Name)
	FlagDaemonLogLevel(flags, &options.Level)
	cmd.MarkFlagRequired("name")
	return cmd
}

func (t *CmdDaemonListenerLog) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonListenerLogControl(ctx, nodename, t.Name, api.PostDaemonListenerLogControlJSONRequestBody{Level: t.Level})
	}
	return t.CmdDaemonSubAction.Run(fn)
}
