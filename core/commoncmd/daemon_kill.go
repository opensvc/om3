package commoncmd

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/spf13/cobra"
)

type (
	CmdDaemonKill struct {
		CmdDaemonSubAction
		Pid    []int
		Signal string
	}
)

func NewCmdDaemonKill() *cobra.Command {
	options := CmdDaemonKill{}
	cmd := &cobra.Command{
		Use:   "kill PID...",
		Short: "kill a running process",
		Long: `Send a signal to processes the daemon has spawned.

The killable processes are the ones the 'daemon ps' command lists. Any other
pid is refused. As a pid only makes sense on the node the process runs on, use
--node to signal a process listed on a peer node. The default is the local
node.

Example:

	om daemon ps
	om daemon kill 3924688 --signal=term
`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				pid, err := strconv.Atoi(arg)
				if err != nil {
					return fmt.Errorf("invalid pid %s", arg)
				}
				options.Pid = append(options.Pid, pid)
			}
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	flags.StringVar(&options.Signal, "signal", "", "the signal to send, as a name (term, sigterm) or a number (15) (default \"kill\")")
	return cmd
}

func (t *CmdDaemonKill) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		params := &api.DeleteDaemonProcessParams{
			Pid: &t.Pid,
		}
		if t.Signal != "" {
			params.Signal = &t.Signal
		}
		return c.DeleteDaemonProcess(ctx, nodename, params)
	}
	return t.CmdDaemonSubAction.Run(fn)
}
