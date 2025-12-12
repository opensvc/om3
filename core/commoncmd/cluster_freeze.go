package commoncmd

import (
	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/spf13/cobra"
)

type CmdClusterFreeze struct {
	OptsAsync
	Color  string
	Output string
}

func NewCmdClusterFreeze() *cobra.Command {
	var options CmdClusterFreeze
	cmd := &cobra.Command{
		GroupID: GroupIDOrchestratedActions,
		Use:     "freeze",
		Short:   "block ha automatic start and split action on all nodes",
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

func (t *CmdClusterFreeze) Run() error {
	return nodeaction.New(
		nodeaction.WithAsyncTarget("frozen"),
		nodeaction.WithAsyncTime(t.Time),
		nodeaction.WithAsyncWait(t.Wait),
		nodeaction.WithAsyncWatch(t.Watch),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocal(false),
	).Do()
}
