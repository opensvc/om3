package commoncmd

import (
	"context"
	"net/http"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/spf13/cobra"
)

type (
	CmdDaemonKill struct {
		CmdDaemonSubAction
		Pid []int
	}
)

func NewCmdDaemonKill() *cobra.Command {
	options := CmdDaemonKill{}
	cmd := &cobra.Command{
		Use:   "kill",
		Short: "kill a running process",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	flags.IntSliceVar(&options.Pid, "pid", []int{}, "the pid of the process to kill")
	return cmd
}

func (t *CmdDaemonKill) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		params := &api.DeleteDaemonProcessParams{
			Pid: &t.Pid,
		}
		return c.DeleteDaemonProcess(ctx, nodename, params)
	}
	return t.CmdDaemonSubAction.Run(fn)
}
