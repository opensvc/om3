package commoncmd

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
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

		// Resolve returns the id the ID given names, which can be the
		// start of one, as the listings and the status show them.
		Resolve func(t *CmdDaemonIDLogs) (string, error)
	}
)

func (t *CmdDaemonIDLogs) Run() error {
	if t.ID == "" {
		return fmt.Errorf("no id to report the logs of")
	}
	if t.Resolve != nil {
		// A log entry carries the whole id, which the filter matches
		// whole: the start of one would match nothing, silently.
		id, err := t.Resolve(t)
		if err != nil {
			return err
		}
		t.ID = id
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
// idWithPrefix returns the one id of ids starting with prefix.
func idWithPrefix(kind, prefix string, ids []string) (string, error) {
	matches := make([]string, 0)
	for _, id := range ids {
		if strings.HasPrefix(id, prefix) && !slices.Contains(matches, id) {
			matches = append(matches, id)
		}
	}
	switch len(matches) {
	case 0:
		// The log entries outlive what the daemons remember of the ids,
		// and a whole id still reads them.
		return "", fmt.Errorf("no %s id the daemons know starts with %s: a daemon forgets an id an hour after what it names ended, or when it restarts, and the whole id is still read", kind, prefix)
	case 1:
		return matches[0], nil
	default:
		slices.Sort(matches)
		return "", fmt.Errorf("%s starts %d %s ids, %s: give more of the id", prefix, len(matches), kind, strings.Join(matches, " "))
	}
}

// ResolveOrchestrationID returns the orchestration id s names: an id, or the
// start of the one id the daemon answering knows, as the status shows it.
// Any node answers for any orchestration.
func ResolveOrchestrationID(s string) (string, error) {
	if _, err := uuid.Parse(s); err == nil {
		return s, nil
	}
	c, err := client.New()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.GetDaemonOrchestrationsWithResponse(ctx, "_", &api.GetDaemonOrchestrationsParams{})
	if err != nil {
		return "", err
	}
	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		return "", fmt.Errorf("list the orchestrations: %s", resp.Status())
	}
	ids := make([]string, len(resp.JSON200.Items))
	for i, item := range resp.JSON200.Items {
		ids[i] = item.OrchestrationID
	}
	return idWithPrefix("orchestration", s, ids)
}

// ResolveSessionID returns the session id s names: an id, or the start of
// the one id the daemons of the selected nodes know.
func ResolveSessionID(nodeSelector, s string) (string, error) {
	return resolveExecID(nodeSelector, s, "session", func(item api.ExecItem) string { return item.SessionID })
}

// ResolveExecID returns the exec id s names: an id, or the start of the one
// id the daemons of the selected nodes know.
func ResolveExecID(nodeSelector, s string) (string, error) {
	return resolveExecID(nodeSelector, s, "exec", func(item api.ExecItem) string { return item.ExecID })
}

func resolveExecID(nodeSelector, s, kind string, idOf func(api.ExecItem) string) (string, error) {
	if _, err := uuid.Parse(s); err == nil {
		return s, nil
	}
	items, err := (&CmdDaemonExecList{NodeSelector: nodeSelector}).Gather()
	if err != nil && len(items) == 0 {
		return "", err
	}
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = idOf(item)
	}
	return idWithPrefix(kind, s, ids)
}

func newCmdDaemonIDLogs(kind, key, arg, long string, resolve func(*CmdDaemonIDLogs) (string, error)) *cobra.Command {
	options := CmdDaemonIDLogs{Key: key, Resolve: resolve}
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

"om daemon orchestration list" reports the orchestrations and their ids. The
start of an id, as the status shows it, names the one it starts.`, func(t *CmdDaemonIDLogs) (string, error) {
			return ResolveOrchestrationID(t.ID)
		})
}

// NewCmdDaemonSessionLogs returns the logs of one session.
func NewCmdDaemonSessionLogs() *cobra.Command {
	return newCmdDaemonIDLogs("session", "SESSION_ID", "SESSION_ID",
		`Show what was logged under one session, on every node it reached.

A session is one submitted command, whole. It may have reached several
objects and several nodes, each of them an exec with its own log entries,
and this is all of them.

"om daemon session list" reports the sessions and their ids. The start of an
id names the one it starts.`, func(t *CmdDaemonIDLogs) (string, error) {
			return ResolveSessionID(t.NodeSelector, t.ID)
		})
}

// NewCmdDaemonExecLogs returns the logs of one exec.
func NewCmdDaemonExecLogs() *cobra.Command {
	return newCmdDaemonIDLogs("exec", "EXEC_ID", "EXEC_ID",
		`Show what was logged under one exec.

An exec is one run of one command, on one node, for one object. It is the
finest of the three ids: the session it belongs to has the entries of its
siblings too, and the orchestration above that has the entries of every node.

"om daemon exec list" reports the execs and their ids. The start of an id
names the one it starts.`, func(t *CmdDaemonIDLogs) (string, error) {
			return ResolveExecID(t.NodeSelector, t.ID)
		})
}
