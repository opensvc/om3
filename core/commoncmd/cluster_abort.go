package commoncmd

import (
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/spf13/cobra"
)

type CmdClusterAbort struct {
	Color  string
	Output string
	OptsAsync
}

func NewCmdClusterAbort() *cobra.Command {
	var options CmdClusterAbort
	cmd := &cobra.Command{
		GroupID: GroupIDOrchestratedActions,
		Use:     "abort",
		Short:   "abort the running orchestration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagColor(flags, &options.Color)
	FlagOutput(flags, &options.Output)
	FlagsAsync(flags, &options.OptsAsync)
	return cmd
}

func (t *CmdClusterAbort) Run() error {
	return nodeaction.New(
		nodeaction.WithAsyncTarget("aborted"),
		nodeaction.WithAsyncWatch(t.Watch),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
	).Do()
}
