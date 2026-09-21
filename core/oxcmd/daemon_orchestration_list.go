package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/session"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdDaemonOrchestrationList struct {
		OptsGlobal
		NodeSelector    string
		States          []string
		OrchestrationID string

		// Wait holds each request until the orchestration has ended, and is
		// how "om daemon orchestration wait" waits: the daemon answers when
		// the orchestration is over rather than when it is asked.
		Wait time.Duration

		// Unbounded says the caller named no duration and waits for as long
		// as it takes. The daemon holds one request for an hour at most, so
		// that wait is the request asked again until the orchestration ends.
		Unbounded bool
	}
)

func (t *CmdDaemonOrchestrationList) Run() error {
	items, err := t.run()
	t.render(t.filter(items))
	return err
}

// RunWait waits for the orchestration to end, reports it, and says whether it
// did what it was for.
//
// The orchestration id is what the action the client submitted was answered
// with, and every node answers for it, so this is how a client follows an
// action it asked of a node it can no longer name, or that it never named.
func (t *CmdDaemonOrchestrationList) RunWait() error {
	if t.OrchestrationID == "" {
		return fmt.Errorf("an orchestration id is required")
	}
	items, err := t.runWaiting()
	t.render(items)
	if err != nil {
		return err
	}
	for _, item := range items {
		switch item.State {
		case string(session.StateSucceeded):
			return nil
		default:
			if item.Error != nil && *item.Error != "" {
				return fmt.Errorf("orchestration %s %s: %s", t.OrchestrationID, item.State, *item.Error)
			}
			return fmt.Errorf("orchestration %s %s", t.OrchestrationID, item.State)
		}
	}
	return nil
}

// runWaiting asks, and asks again for as long as the caller is prepared to
// wait: the daemon holds one request for an hour at most, and a wait with no
// duration is that hour asked again until the orchestration ends.
func (t *CmdDaemonOrchestrationList) runWaiting() ([]api.OrchestrationItem, error) {
	until := time.Time{}
	if !t.Unbounded {
		until = time.Now().Add(t.Wait)
	}
	if t.Wait > commoncmd.DefaultWait {
		t.Wait = commoncmd.DefaultWait
	}
	for {
		items, err := t.run()
		if commoncmd.IsStillRunning(err) && t.keepWaiting(until) {
			// The hold expired, not the wait: ask again.
			continue
		}
		return items, err
	}
}

// keepWaiting says the wait is not over, and narrows the next request to what
// is left of it.
func (t *CmdDaemonOrchestrationList) keepWaiting(until time.Time) bool {
	if t.Unbounded {
		return true
	}
	remaining := time.Until(until)
	if remaining <= 100*time.Millisecond {
		return false
	}
	if remaining < t.Wait {
		t.Wait = remaining
	}
	return true
}

func (t *CmdDaemonOrchestrationList) run() ([]api.OrchestrationItem, error) {
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

	items, errs := t.merge(t.gather(c, nodenames))

	// Every node of the object answers for an orchestration, so one that
	// has forgotten it is only an answer when they all have.
	if t.OrchestrationID != "" && len(items) == 0 {
		if errs != nil {
			return nil, errs
		}
		return nil, fmt.Errorf("orchestration %s is no longer known on %s: it ended long enough ago to have been dropped, or never ran there",
			t.OrchestrationID, t.NodeSelector)
	}
	return items, errs
}

// orchestrationListSort is newest first, as the exec and session listings
// are: the one asked about is the one that just ran.
const orchestrationListSort = "-started_at,orchestration_id"

// filter narrows what the nodes answered the way the daemon would have.
//
// Naming an orchestration id asks the by-id endpoint, which answers for the
// id and knows nothing of the options that narrow a listing. Applying them
// here is what makes them mean the same thing with an id as without one.
// Whether the daemon still holds the id has already been answered by then, so
// an empty result at this point is the options, not forgetting.
func (t *CmdDaemonOrchestrationList) filter(items []api.OrchestrationItem) []api.OrchestrationItem {
	if len(t.States) == 0 {
		return items
	}
	l := make([]api.OrchestrationItem, 0, len(items))
	for _, i := range items {
		if slices.Contains(t.States, i.State) {
			l = append(l, i)
		}
	}
	return l
}

// merge folds the answers of the nodes into one entry per orchestration.
//
// Every node of an object answers for its orchestrations, so asking several
// returns the same orchestration several times. The answer of the node that
// accepted it is the one kept, being the only one that knows which node was
// asked for it.
func (t *CmdDaemonOrchestrationList) merge(items []api.OrchestrationItem, errs error) ([]api.OrchestrationItem, error) {
	byID := make(map[string]api.OrchestrationItem, len(items))
	order := make([]string, 0, len(items))
	for _, i := range items {
		kept, ok := byID[i.OrchestrationID]
		if !ok {
			byID[i.OrchestrationID] = i
			order = append(order, i.OrchestrationID)
			continue
		}
		if kept.Node == "" && i.Node != "" {
			byID[i.OrchestrationID] = i
		}
	}
	l := make([]api.OrchestrationItem, 0, len(order))
	for _, id := range order {
		l = append(l, byID[id])
	}
	return l, errs
}

// gather asks every node, and returns what they answered together with what
// went wrong asking. A node that has forgotten the id is not one of the
// things that went wrong.
func (t *CmdDaemonOrchestrationList) gather(c *client.T, nodenames []string) ([]api.OrchestrationItem, error) {
	// The requests are held for as long as the wait asks, and the grace on
	// top of it is for the round trip.
	timeout := 5 * time.Second
	if t.Wait > 0 {
		timeout = t.Wait + 5*time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var (
		mu    sync.Mutex
		items []api.OrchestrationItem
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
	return items, errs
}

func (t *CmdDaemonOrchestrationList) one(ctx context.Context, c *client.T, nodename string) ([]api.OrchestrationItem, error) {
	if t.OrchestrationID != "" {
		params := api.GetDaemonOrchestrationParams{}
		if t.Wait > 0 {
			wait := t.Wait.String()
			params.Wait = &wait
		}
		resp, err := c.GetDaemonOrchestrationWithResponse(ctx, nodename, t.OrchestrationID, &params)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", nodename, err)
		}
		switch resp.StatusCode() {
		case http.StatusOK:
			return []api.OrchestrationItem{*resp.JSON200}, nil
		case http.StatusRequestTimeout:
			return nil, fmt.Errorf("%s: orchestration %s is %w", nodename, t.OrchestrationID, commoncmd.ErrStillRunning)
		case http.StatusGone:
			// This node has forgotten it, or never ran it. Another may hold
			// it, and saying so here would make asking every node an error
			// wherever one of them answers.
			return nil, nil
		default:
			return nil, fmt.Errorf("%s: %s", nodename, resp.Status())
		}
	}

	params := api.GetDaemonOrchestrationsParams{}
	if len(t.States) > 0 {
		params.States = &t.States
	}
	resp, err := c.GetDaemonOrchestrationsWithResponse(ctx, nodename, &params)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", nodename, err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", nodename, resp.Status())
	}
	return resp.JSON200.Items, nil
}

func (t *CmdDaemonOrchestrationList) render(items []api.OrchestrationItem) error {
	return output.Renderer{
		DefaultSort:   orchestrationListSort,
		Sort:          t.Sort,
		DefaultOutput: "tab=STATE:state,ORCHESTRATION_ID:orchestration_id,PATH:path,EXPECT:expect,ACCEPTED_BY:node,STARTED_AT:started_at,DURATION:duration",
		Output:        t.Output,
		Color:         t.Color,
		Data:          toOrchestrationViews(items),
		Colorize:      rawconfig.Colorize,
	}.Print()
}

// orchestrationView is what the table shows, for the reason sessionView is.
type orchestrationView struct {
	State           string     `json:"state"`
	OrchestrationID string     `json:"orchestration_id"`
	Path            string     `json:"path,omitempty"`
	Expect          string     `json:"expect,omitempty"`
	AcceptedBy      string     `json:"node,omitempty"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	Duration        string     `json:"duration,omitempty"`
	Error           string     `json:"error,omitempty"`
}

func toOrchestrationViews(items []api.OrchestrationItem) []orchestrationView {
	now := time.Now()
	l := make([]orchestrationView, 0, len(items))
	for _, i := range items {
		v := orchestrationView{
			State:           i.State,
			OrchestrationID: i.OrchestrationID,
			AcceptedBy:      i.Node,
			StartedAt:       i.StartedAt.Truncate(time.Second),
		}
		if i.Path != nil {
			v.Path = *i.Path
		}
		if i.Expect != nil {
			v.Expect = *i.Expect
		}
		if i.Error != nil {
			v.Error = *i.Error
		}
		if i.EndedAt != nil {
			endedAt := i.EndedAt.Truncate(time.Second)
			v.EndedAt = &endedAt
		}
		v.Duration = commoncmd.RenderDuration(i.StartedAt, i.EndedAt, now)
		l = append(l, v)
	}
	return l
}
