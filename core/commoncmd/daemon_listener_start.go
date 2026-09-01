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
	CmdDaemonListenerStart struct {
		CmdDaemonSubAction
		Name string
	}
)

func NewCmdDaemonListenerStart() *cobra.Command {
	options := CmdDaemonListenerStart{}
	cmd := &cobra.Command{
		Use:               "start NAME",
		Short:             fmt.Sprintf("start a daemon a listener"),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: validListenerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Name = args[0]
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func (t *CmdDaemonListenerStart) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonListenerStart(ctx, nodename, api.InPathListenerName(t.Name))
	}
	return t.CmdDaemonSubAction.Run(fn)
}
