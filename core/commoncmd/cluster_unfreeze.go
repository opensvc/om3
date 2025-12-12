package commoncmd

import (
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/nodeaction"
)

type CmdClusterUnfreeze struct {
	OptsAsync
	Color  string
	Output string
}

func NewCmdClusterUnfreeze() *cobra.Command {
	var options CmdClusterUnfreeze
	cmd := &cobra.Command{
		GroupID: GroupIDOrchestratedActions,
		Use:     "unfreeze",
		Hidden:  false,
		Short:   "unblock ha automatic and split action start on all nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagsAsync(flags, &options.OptsAsync)
	FlagColor(flags, &options.Color)
	FlagOutput(flags, &options.Output)
	return cmd
}

func (t *CmdClusterUnfreeze) Run() error {
	return nodeaction.New(
		nodeaction.WithAsyncTarget("unfrozen"),
		nodeaction.WithAsyncTime(t.Time),
		nodeaction.WithAsyncWait(t.Wait),
		nodeaction.WithAsyncWatch(t.Watch),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocal(false),
	).Do()
}

func NewCmdClusterThaw() *cobra.Command {
	cmd := NewCmdClusterUnfreeze()
	cmd.Use = "thaw"
	cmd.Hidden = true
	return cmd
}
