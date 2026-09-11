package commoncmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	// CmdDaemonSessionList reports sessions, one row per session.
	//
	// A session is what was submitted; an exec is one run of one object on
	// one node, and one session is several of them. Folding the execs into
	// the session is the work this command exists to do: a reader asking how
	// their command went should not have to count succeeded rows themselves.
	//
	// The fold is done here and not in the daemon because a session spans
	// nodes and each daemon holds only the execs it ran. A per-node session
	// is a partial one, which is worse than none.
	CmdDaemonSessionList struct {
		CmdDaemonExecList
	}

	// SessionView is one session, folded from its execs.
	SessionView struct {
		startedAt       time.Time
		SessionID       string `json:"session_id"`
		State           string `json:"state"`
		OrchestrationID string `json:"orchestration_id,omitempty"`
		Execs           int    `json:"execs"`
		Running         int    `json:"running"`
		Failed          int    `json:"failed"`
		Nodes           int    `json:"nodes"`
		Objects         int    `json:"objects"`
		Origin          string `json:"origin"`
		StartedAt       string `json:"started_at"`
		EndedAt         string `json:"ended_at,omitempty"`
		Duration        string `json:"duration,omitempty"`
		Command         string `json:"command,omitempty"`
	}
)

const sessionListColumns = "tab=SESSION_ID:session_id,STATE:state,EXECS:execs,FAILED:failed,NODES:nodes,OBJECTS:objects,ORIGIN:origin,STARTED_AT:started_at,DURATION:duration,COMMAND:command"

func (t *CmdDaemonSessionList) Run() error {
	items, err := t.Gather()
	// A node that could not be reached is reported, but the fold of what the
	// others answered is still shown, with the counts it is a fold of.
	output.Renderer{
		DefaultOutput: sessionListColumns,
		Output:        t.Output,
		Color:         t.Color,
		Data:          ToSessionViews(items),
		Colorize:      rawconfig.Colorize,
	}.Print()
	return err
}

// ToSessionViews folds execs into the sessions they belong to, newest first.
func ToSessionViews(items []api.ExecItem) []SessionView {
	type acc struct {
		v       SessionView
		begin   time.Time
		end     time.Time
		ended   bool
		nodes   map[string]bool
		objects map[string]bool
		command string
		varies  bool
	}
	byID := make(map[string]*acc)
	order := make([]string, 0)
	for _, i := range items {
		a, ok := byID[i.SessionID]
		if !ok {
			a = &acc{
				v:       SessionView{SessionID: i.SessionID, Origin: i.Origin},
				begin:   i.StartedAt,
				nodes:   make(map[string]bool),
				objects: make(map[string]bool),
				command: i.Command,
			}
			if i.OrchestrationID != nil {
				a.v.OrchestrationID = *i.OrchestrationID
			}
			byID[i.SessionID] = a
			order = append(order, i.SessionID)
		}
		a.v.Execs++
		switch i.State {
		case "running":
			a.v.Running++
		case "failed":
			a.v.Failed++
		}
		a.nodes[i.Node] = true
		if i.Path != nil {
			a.objects[*i.Path] = true
		}
		if i.Command != a.command {
			a.varies = true
		}
		if i.StartedAt.Before(a.begin) {
			a.begin = i.StartedAt
		}
		if i.EndedAt != nil {
			a.ended = true
			if i.EndedAt.After(a.end) {
				a.end = *i.EndedAt
			}
		}
	}

	now := time.Now()
	l := make([]SessionView, 0, len(order))
	for _, id := range order {
		a := byID[id]
		v := a.v
		v.Nodes = len(a.nodes)
		v.Objects = len(a.objects)
		// The state of the whole, which is the fold a reader would otherwise
		// do by eye: unfinished while any exec runs, failed if any failed,
		// and succeeded only when every one of them did.
		switch {
		case v.Running > 0:
			v.State = "running"
		case v.Failed > 0:
			v.State = "failed"
		default:
			v.State = "succeeded"
		}
		v.startedAt = a.begin
		v.StartedAt = a.begin.Truncate(time.Second).Format(time.RFC3339)
		// Wall time of the whole command, not the sum of its parts: the execs
		// run at the same time, and what the submitter waited is the span.
		if v.Running == 0 && a.ended {
			v.EndedAt = a.end.Truncate(time.Second).Format(time.RFC3339)
			v.Duration = RenderDuration(a.begin, &a.end, now)
		} else {
			v.Duration = RenderDuration(a.begin, nil, now)
		}
		v.Command = a.command
		if a.varies {
			// The execs of one session run different commands when it reached
			// several objects. Naming one of them and counting the rest beats
			// naming one as if it were all of them.
			v.Command = fmt.Sprintf("%s (+%d)", a.command, v.Execs-1)
		}
		l = append(l, v)
	}
	sort.Slice(l, func(i, j int) bool { return l[i].startedAt.After(l[j].startedAt) })
	return l
}

// NewCmdDaemonSessionList returns the listing of sessions.
//
// It defaults to every node, where the exec listing defaults to this one: a
// session spans the nodes it reached, and folding only the local share of it
// would report a count that is not the session's.
func NewCmdDaemonSessionList() *cobra.Command {
	options := CmdDaemonSessionList{}
	cmd := &cobra.Command{
		Use:   "list [SESSION_ID]",
		Short: "list the commands submitted to the daemon, and how they went",
		Long: `A session is one submitted command, whole. It may have reached several
objects and several nodes, each of them an exec with its own outcome, and
what is reported here is the fold of those: how many ran, how many failed,
and the state of the whole.

"om daemon exec list --session-id ID" shows the execs one by one.

Every node is asked, because a session spans the nodes it reached and the
count of a part of one is not the count of the session.`,
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				options.SessionID = args[0]
			}
			return options.Run()
		},
	}
	CmdWithArg(cmd, `SESSION_ID  The session id the submitter of the action was handed.`)
	flags := cmd.Flags()
	FlagNodeSelectorWithDefault(flags, &options.NodeSelector, "*")
	FlagOutput(flags, &options.Output)
	FlagColor(flags, &options.Color)
	FlagObjectSelector(flags, &options.Selector)
	flags.StringSliceVar(&options.Origins, "origin", nil, "list the sessions this submitted, every submitter when not set (api, imon, nmon, scheduler)")
	flags.StringVar(&options.OrchestrationID, "orchestration-id", "", "list the sessions run under this orchestration")
	flags.StringSliceVar(&options.States, "state", nil, "list the execs in these states before folding them, every state when not set")
	return cmd
}
