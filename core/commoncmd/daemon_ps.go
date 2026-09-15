package commoncmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// NewCmdDaemonPs returns the listing of what the daemon is running now.
//
// It is CmdDaemonExecList with the running state preselected and the pid
// leading the table. What is running and what has run are the same records at
// different ages, so there is one implementation and this is a preset of it,
// rather than a second command that can drift from the first.
func NewCmdDaemonPs() *cobra.Command {
	options := CmdDaemonExecList{
		States:  []string{"running"},
		Columns: execPsColumns,
	}
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "list what the daemon is running now",
		Long: `List the commands the daemon has started and not yet reaped, with the pid of
each, which is what "om daemon kill" signals.

This is "om daemon exec list --state running" under another name. Drop the
state to see what has run as well.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	FlagOutput(flags, &options.Output)
	FlagSort(flags, &options.Sort)
	FlagColor(flags, &options.Color)
	FlagObjectSelector(flags, &options.Selector)
	FlagRID(flags, &options.RID)
	FlagDaemonExecFilters(flags, &options)
	return cmd
}

// FlagDaemonExecFilters declares the options that narrow a listing of execs,
// on the listing and on every preset of it, so the same question is asked the
// same way wherever it is asked.
func FlagDaemonExecFilters(flags *pflag.FlagSet, options *CmdDaemonExecList) {
	flags.StringSliceVar(&options.Origins, "origin", nil, "list the execs this submitted, every submitter when not set (api, imon, nmon, scheduler)")
	flags.StringVar(&options.SessionID, "session-id", "", "list the execs of this session, which is the whole of one submitted command")
	flags.StringVar(&options.OrchestrationID, "orchestration-id", "", "list the execs run under this orchestration")
}
