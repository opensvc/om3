package commoncmd

import (
	"context"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdDaemonListenerRestart struct {
		CmdDaemonSubAction
		Name string
	}
)

func NewCmdDaemonListenerRestart() *cobra.Command {
	options := CmdDaemonListenerRestart{}
	cmd := &cobra.Command{
		Use:   "restart NAME",
		Short: "restart a daemon listener",
		Long: ForProgram("Stop then start one of the listeners the daemon serves the api with.\n\n" +
			ListenerNameHelp),
		Example: ForProgram(`  # restart the tcp listener on the local node
  om daemon listener restart api.inet

  # restart it on every node
  om daemon listener restart api.inet --node '*'`),
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

func (t *CmdDaemonListenerRestart) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonListenerRestart(ctx, nodename, api.InPathListenerName(t.Name))
	}
	return t.CmdDaemonSubAction.Run(fn)
}
