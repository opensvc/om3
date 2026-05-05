package commoncmd

import (
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/util/render"
)

type (
	CmdClusterLogs struct {
		OptsLogs
		Color  string
		Output string
	}
)

func NewCmdClusterLogs() *cobra.Command {
	var options CmdClusterLogs
	cmd := &cobra.Command{
		GroupID: GroupIDQuery,
		Use:     "logs",
		Aliases: []string{"logs", "log", "lo"},
		Short:   "show all nodes logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagColor(flags, &options.Color)
	FlagOutput(flags, &options.Output)
	FlagsLogs(flags, &options.OptsLogs)
	return cmd
}

func (t *CmdClusterLogs) Run() error {
	render.SetColor(t.Color)
	return t.asCommonCmd().Remote()
}

func (t *CmdClusterLogs) asCommonCmd() *CmdNodeLogs {
	return &CmdNodeLogs{
		OptsGlobal: OptsGlobal{
			Color:  t.Color,
			Output: t.Output,
		},
		OptsLogs: OptsLogs{
			Follow: t.Follow,
			Lines:  t.Lines,
			Filter: t.Filter,
			Grep:   t.Grep,
		},
		NodeSelector: "*",
	}
}
