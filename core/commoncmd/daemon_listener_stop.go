package commoncmd

import (
	"context"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdDaemonListenerStop struct {
		CmdDaemonSubAction
		Name string
	}
)

func NewCmdDaemonListenerStop() *cobra.Command {
	options := CmdDaemonListenerStop{}
	cmd := &cobra.Command{
		Use:   "stop NAME",
		Short: "stop a daemon listener",
		Long: ForProgram("Stop one of the listeners the daemon serves the api with.\n\n" +
			ListenerNameHelp),
		Example: ForProgram(`  # stop the tcp listener on the local node
  om daemon listener stop api.inet

  # stop it on every node
  om daemon listener stop api.inet --node '*'`),
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

func (t *CmdDaemonListenerStop) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonListenerStop(ctx, nodename, api.InPathListenerName(t.Name))
	}
	return t.CmdDaemonSubAction.Run(fn)
}
