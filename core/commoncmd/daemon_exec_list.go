package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/duration"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	// CmdDaemonExecList lists the commands the daemon ran and is running.
	//
	// It is the one listing of the exec store, and "om daemon ps" is it with
	// the running state preselected: what is running and what has run are the
	// same records at different ages, so they are read by one code path and
	// shown in one shape.
	CmdDaemonExecList struct {
		NodeSelector    string
		Output          string
		Color           string
		States          []string
		Origins         []string
		SessionID       string
		OrchestrationID string
		ExecID          string
		RID             string
		Selector        string

		// Columns is the table this renders. "om daemon ps" leads with the
		// pid, the listing leads with the outcome.
		Columns string

		// Sort overrides the order the listing comes in.
		Sort string
	}
)

const (
	// execListSort is newest first, then the execs of one command together,
	// then the objects of one node in a stable order. The union of what
	// several nodes answered arrives in whatever order they answered in, so
	// without this the rows are ordered by nothing at all.
	execListSort = "-started_at,session_id,node,path"

	execListColumns = "tab=NODE:node,STATE:state,EXEC_ID:exec_id,SESSION_ID:session_id,PATH:path,ORIGIN:origin,STARTED_AT:started_at,DURATION:duration,COMMAND:command"
	execPsColumns   = "tab=PID:pid,NODE:node,EXEC_ID:exec_id,PATH:path,RID:rid,ORIGIN:origin,DURATION:duration,COMMAND:command"
)

func (t *CmdDaemonExecList) Run() error {
	items, err := t.Gather()
	// A node that could not be reached is reported, but what the others
	// answered is still shown: a listing of most of the cluster beats none of
	// it.
	err = errors.Join(err, t.render(items))
	return err
}

// Gather asks every selected node and returns what they answered together.
//
// An exec is node-local, so the union over the nodes is the whole of what a
// command did. A node that has forgotten an id it is asked for by name is not
// an error when another still holds it, which is why the by-id answers are
// folded in rather than returned.
func (t *CmdDaemonExecList) Gather() ([]api.ExecItem, error) {
	c, err := client.New()
	if err != nil {
		return nil, err
	}
	if t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return nil, err
	}
	if len(nodenames) == 0 {
		return nil, fmt.Errorf("no node matching %s", t.NodeSelector)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		mu    sync.Mutex
		items []api.ExecItem
		errs  error
		wg    sync.WaitGroup
	)
	for _, nodename := range nodenames {
		wg.Add(1)
		go func(nodename string) {
			defer wg.Done()
			l, err := t.one(ctx, c, nodename)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = errors.Join(errs, err)
				return
			}
			items = append(items, l...)
		}(nodename)
	}
	wg.Wait()

	if errs != nil {
		return items, errs
	}
	if len(items) == 0 {
		// Only an id the caller named can be reported as forgotten. An empty
		// listing is an empty listing.
		switch {
		case t.ExecID != "":
			return nil, fmt.Errorf("exec %s is no longer known on %s: it ended long enough ago to have been dropped, or never ran there", t.ExecID, t.NodeSelector)
		case t.SessionID != "":
			return nil, fmt.Errorf("session %s is no longer known on %s: it ended long enough ago to have been dropped, or never ran there", t.SessionID, t.NodeSelector)
		}
	}
	return items, nil
}

func (t *CmdDaemonExecList) one(ctx context.Context, c *client.T, nodename string) ([]api.ExecItem, error) {
	if t.ExecID != "" {
		execID, err := uuid.Parse(t.ExecID)
		if err != nil {
			return nil, fmt.Errorf("exec id %s: %w", t.ExecID, err)
		}
		resp, err := c.GetDaemonExecWithResponse(ctx, nodename, execID)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", nodename, err)
		}
		switch resp.StatusCode() {
		case http.StatusOK:
			return t.filter([]api.ExecItem{*resp.JSON200}), nil
		case http.StatusGone:
			// This node has forgotten it, or never ran it. Another may hold
			// it, and saying so here would make asking every node an error
			// wherever one of them answers.
			return nil, nil
		default:
			return nil, fmt.Errorf("%s: %s", nodename, resp.Status())
		}
	}

	params := api.GetDaemonExecsParams{}
	if len(t.States) > 0 {
		params.States = &t.States
	}
	if len(t.Origins) > 0 {
		params.Origins = &t.Origins
	}
	if t.SessionID != "" {
		id, err := uuid.Parse(t.SessionID)
		if err != nil {
			return nil, fmt.Errorf("session id %s: %w", t.SessionID, err)
		}
		params.SessionID = &id
	}
	if t.OrchestrationID != "" {
		id, err := uuid.Parse(t.OrchestrationID)
		if err != nil {
			return nil, fmt.Errorf("orchestration id %s: %w", t.OrchestrationID, err)
		}
		params.OrchestrationID = &id
	}
	if t.RID != "" {
		params.RID = &t.RID
	}
	if t.Selector != "" {
		params.Selector = &t.Selector
	}
	resp, err := c.GetDaemonExecsWithResponse(ctx, nodename, &params)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", nodename, err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", nodename, resp.Status())
	}
	return resp.JSON200.Items, nil
}

// filter narrows what a by-id answer returned the way the daemon would have.
//
// Naming an exec id asks the by-id endpoint, which answers for the id and
// knows nothing of the options that narrow a listing. Applying them here is
// what makes them mean the same thing with an id as without one.
func (t *CmdDaemonExecList) filter(items []api.ExecItem) []api.ExecItem {
	l := make([]api.ExecItem, 0, len(items))
	for _, i := range items {
		if t.SessionID != "" && i.SessionID != t.SessionID {
			continue
		}
		if t.OrchestrationID != "" && (i.OrchestrationID == nil || *i.OrchestrationID != t.OrchestrationID) {
			continue
		}
		if len(t.States) > 0 && !slices.Contains(t.States, i.State) {
			continue
		}
		if len(t.Origins) > 0 && !slices.Contains(t.Origins, i.Origin) {
			continue
		}
		l = append(l, i)
	}
	return l
}

func (t *CmdDaemonExecList) render(items []api.ExecItem) error {
	columns := t.Columns
	if columns == "" {
		columns = execListColumns
	}
	return output.Renderer{
		DefaultOutput: columns,
		Output:        t.Output,
		DefaultSort:   execListSort,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          ToExecViews(items),
		Colorize:      rawconfig.Colorize,
	}.Print()
}

// ExecView is what the table shows: the api reports a duration in nanoseconds
// and an instant to the nanosecond, which a machine wants and a reader does
// not.
type ExecView struct {
	Node            string     `json:"node"`
	State           string     `json:"state"`
	ExecID          string     `json:"exec_id"`
	SessionID       string     `json:"session_id"`
	OrchestrationID string     `json:"orchestration_id,omitempty"`
	Path            string     `json:"path,omitempty"`
	Origin          string     `json:"origin"`
	RID             string     `json:"rid,omitempty"`
	Pid             *int       `json:"pid,omitempty"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	// Duration is rendered, not carried: the two ends are the data. So
	// --sort=duration orders the text of it, where --sort=started_at and
	// --sort=ended_at order the moments they name.
	Duration string `json:"duration,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
	Command  string `json:"command,omitempty"`
	Error    string `json:"error,omitempty"`
}

// RenderDuration is how long a span lasted, or how long it has lasted so far
// when it has not ended.
//
// The records carry the two ends and not the length: two numbers that answer
// the same question can disagree, and these two did. The length is derived
// here, where it is wanted, and derived the same way for an exec, a session
// and an orchestration.
//
// A running span is measured against the reader's clock, where its start came
// from the node that ran it, so a clock skew can make it negative.
// FmtShortDuration answers "0s" for that rather than a negative age.
func RenderDuration(startedAt time.Time, endedAt *time.Time, now time.Time) string {
	if endedAt != nil {
		return duration.FmtShortDuration(endedAt.Sub(startedAt))
	}
	return duration.FmtShortDuration(now.Sub(startedAt))
}

func ToExecViews(items []api.ExecItem) []ExecView {
	l := make([]ExecView, 0, len(items))
	now := time.Now()
	for _, i := range items {
		v := ExecView{
			Node:      i.Node,
			State:     i.State,
			ExecID:    i.ExecID,
			SessionID: i.SessionID,
			Origin:    i.Origin,
			Pid:       i.Pid,
			StartedAt: i.StartedAt.Truncate(time.Second),
			ExitCode:  i.ExitCode,
			Command:   i.Command,
		}
		if i.OrchestrationID != nil {
			v.OrchestrationID = *i.OrchestrationID
		}
		if i.Path != nil {
			v.Path = *i.Path
		}
		if i.RID != nil {
			v.RID = *i.RID
		}
		if i.Error != nil {
			v.Error = *i.Error
		}
		if i.EndedAt != nil {
			endedAt := i.EndedAt.Truncate(time.Second)
			v.EndedAt = &endedAt
		}
		v.Duration = RenderDuration(i.StartedAt, i.EndedAt, now)
		l = append(l, v)
	}
	return l
}

// NewCmdDaemonExecList returns the listing of the exec store.
//
// defaultNodeSelector is what --node defaults to: om answers for the node it
// runs on, ox has no node of its own to prefer.
func NewCmdDaemonExecList(defaultNodeSelector string) *cobra.Command {
	options := CmdDaemonExecList{}
	cmd := &cobra.Command{
		Use:   "list [EXEC_ID]",
		Short: "list the commands the daemon ran, and is running",
		Long: `An exec is one run of one object on one node, which is the scale at which
there is a single outcome, a single duration and a single exit code. One
command reaching several objects of a node is several execs sharing one
session id.

Naming an exec id reports that one run, and says so when the daemon no longer
holds it, which is not the same answer as never having run it.

What is listed is bounded, by age and by count, so a node that has been
running for months does not carry every command it ever ran.`,
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				options.ExecID = args[0]
			}
			return options.Run()
		},
	}
	CmdWithArg(cmd, `EXEC_ID  The exec id the submitter of the action was handed.`)
	flags := cmd.Flags()
	FlagNodeSelectorWithDefault(flags, &options.NodeSelector, defaultNodeSelector)
	FlagOutput(flags, &options.Output)
	FlagSort(flags, &options.Sort)
	FlagColor(flags, &options.Color)
	FlagObjectSelector(flags, &options.Selector)
	FlagRID(flags, &options.RID)
	FlagDaemonExecFilters(flags, &options)
	flags.StringSliceVar(&options.States, "state", nil, "list the execs in these states, every state when not set (running, succeeded, failed)")
	return cmd
}
