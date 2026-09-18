package commoncmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

type (
	// CmdDaemonIDLogs reports the log entries one id names.
	//
	// The daemon stamps what it runs with the ids it was run under: the exec
	// that forked it, the session the exec belongs to, and the orchestration
	// the session was a step of. A listing hands one of those ids back, and
	// this is how it is followed to what was logged under it.
	//
	// It is "om node logs" with the filter already written: the ids are log
	// fields, and matching one is what the filter does.
	CmdDaemonIDLogs struct {
		CmdNodeLogs

		// Key is the log field the id is matched on.
		Key string

		// ID is the id to report the entries of.
		ID string
	}
)

func (t *CmdDaemonIDLogs) Run() error {
	if t.ID == "" {
		return fmt.Errorf("no id to report the logs of")
	}
	// Ahead of the filters the user wrote, which narrow within this id
	// rather than beside it.
	t.Filter = append([]string{fmt.Sprintf("%s=%s", t.Key, t.ID)}, t.Filter...)
	return t.Remote()
}

// newCmdDaemonIDLogs builds the logs command of one kind of id.
//
// Every node is asked, because what one id names is not confined to a node: a
// session reaches the nodes the objects it names are on, and an orchestration
// reaches every node of the object. An exec runs on one node, and which one
// is in the listing rather than in the id, so it is looked for on all of them
// rather than asked for twice.
func newCmdDaemonIDLogs(kind, key, arg, long string) *cobra.Command {
	options := CmdDaemonIDLogs{Key: key}
	cmd := &cobra.Command{
		Use:     "logs " + arg,
		Aliases: []string{"log"},
		Short:   "show the logs of one " + kind,
		Long:    long,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.ID = args[0]
			return options.Run()
		},
	}
	CmdWithArg(cmd, fmt.Sprintf("%s  The %s to report the log entries of.", arg, kind))
	flags := cmd.Flags()
	FlagsLogs(flags, &options.OptsLogs)
	FlagNodeSelectorWithDefault(flags, &options.NodeSelector, "*")
	FlagOutput(flags, &options.Output)
	FlagColor(flags, &options.Color)
	return cmd
}

// NewCmdDaemonOrchestrationLogs returns the logs of one orchestration.
func NewCmdDaemonOrchestrationLogs() *cobra.Command {
	return newCmdDaemonIDLogs("orchestration", "ORCHESTRATION_ID", "ORCHESTRATION_ID",
		`Show what was logged under one orchestration, on every node it reached.

An orchestration is a target state asked of an object or of the nodes, and
the id is the one the submitter of the action was handed. It names every
step taken towards that state, on every node that took one, which is what
makes this different from reading the logs of the node the request was sent
to.

"om daemon orchestration list" reports the orchestrations and their ids.`)
}

// NewCmdDaemonSessionLogs returns the logs of one session.
func NewCmdDaemonSessionLogs() *cobra.Command {
	return newCmdDaemonIDLogs("session", "SESSION_ID", "SESSION_ID",
		`Show what was logged under one session, on every node it reached.

A session is one submitted command, whole. It may have reached several
objects and several nodes, each of them an exec with its own log entries,
and this is all of them.

"om daemon session list" reports the sessions and their ids.`)
}

// NewCmdDaemonExecLogs returns the logs of one exec.
func NewCmdDaemonExecLogs() *cobra.Command {
	return newCmdDaemonIDLogs("exec", "EXEC_ID", "EXEC_ID",
		`Show what was logged under one exec.

An exec is one run of one command, on one node, for one object. It is the
finest of the three ids: the session it belongs to has the entries of its
siblings too, and the orchestration above that has the entries of every node.

"om daemon exec list" reports the execs and their ids.`)
}
