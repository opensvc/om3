package commoncmd

import (
	"time"

	"github.com/spf13/cobra"
)

// DefaultWait is how long a wait holds when none is asked for. It is the
// longest a request is held by the daemon, so the default is "until it ends",
// and a client that wants to give up sooner says so.
const DefaultWait = time.Hour

// NewCmdDaemonExecWait returns the command waiting for the end of an exec.
func NewCmdDaemonExecWait(defaultNodeSelector string) *cobra.Command {
	options := CmdDaemonExecList{}
	cmd := &cobra.Command{
		Use:   "wait EXEC_ID",
		Short: "wait for the end of one run of one object on one node",
		Long: `The daemon holds the request until the exec ends, and answers how it went, so
this neither polls nor keeps an event stream open.

An exec that has already ended is answered at once: the daemon remembers it
for a while, which is what lets a client that lost its connection, or that
asks late, still be told how the work went.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.ExecID = args[0]
			if options.Wait == 0 {
				options.Wait = DefaultWait
			}
			return options.RunWait()
		},
	}
	CmdWithArg(cmd, `EXEC_ID  The exec id the submitter of the action was handed.`)
	flags := cmd.Flags()
	FlagNodeSelectorWithDefault(flags, &options.NodeSelector, defaultNodeSelector)
	FlagOutput(flags, &options.Output)
	FlagColor(flags, &options.Color)
	flags.DurationVar(&options.Wait, "duration", 0, "give up waiting after this duration")
	return cmd
}

// NewCmdDaemonSessionWait returns the command waiting for the end of a
// session.
func NewCmdDaemonSessionWait() *cobra.Command {
	options := CmdDaemonSessionList{}
	cmd := &cobra.Command{
		Use:   "wait SESSION_ID",
		Short: "wait for the end of a submitted command",
		Long: `A session is what was submitted, and an exec is one run of one object on one
node, so a session spans the nodes the command reached. Every node is asked,
each holds the request until its own execs have ended, and what is reported is
the fold of what they answer.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.SessionID = args[0]
			if options.Wait == 0 {
				options.Wait = DefaultWait
			}
			return options.RunWait()
		},
	}
	CmdWithArg(cmd, `SESSION_ID  The session id the submitter of the action was handed.`)
	flags := cmd.Flags()
	FlagNodeSelectorWithDefault(flags, &options.NodeSelector, "*")
	FlagOutput(flags, &options.Output)
	FlagColor(flags, &options.Color)
	flags.DurationVar(&options.Wait, "duration", 0, "give up waiting after this duration")
	return cmd
}
