package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	// CmdDaemonKill signals the processes of the execs a filter selects.
	//
	// It selects them the way "om daemon exec list" selects them, so what
	// that lists is what this signals, and the answer is the same shape: the
	// execs signaled.
	CmdDaemonKill struct {
		NodeSelector    string
		Output          string
		Sort            string
		Color           string
		Signal          string
		DryRun          bool
		ExecID          string
		SessionID       string
		OrchestrationID string
		Origins         []string
		RID             string
		Selector        string
	}
)

func NewCmdDaemonKill() *cobra.Command {
	options := CmdDaemonKill{}
	cmd := &cobra.Command{
		Use:   "kill [EXEC_ID]",
		Short: "signal the processes of running execs",
		Long: `Send a signal to the processes of the running execs a filter selects.

The execs are selected the way "om daemon exec list" selects them, so name
what you have: the session id you were handed when you submitted, the
orchestration id, or the object.

There is no pid option, on purpose. Naming an exec rather than a pid is what
makes signaling the wrong process impossible: a pid a listing handed you may
have exited and been recycled since, where an exec id is resolved to its pid
by the daemon at the moment it signals. "om daemon ps" reports the pid of
everything running, and killing a pid is what the system's own kill is for.

Something must narrow the selection. Signaling every exec of a node is not
something you do by leaving the options out. Use --dry-run to see what would
be signaled.

Only the processes the daemon started are signaled, and only the ones still
running: an exec that has ended selects nothing, which is the outcome asked
for.

Example:

	om daemon kill --session-id 723906bb-90a4-400f-8e1f-298227e4838f --node '*'
	om daemon kill --orchestration-id 934a42f9-b7eb-4d7e-a3ac-18347522f9f9
	om daemon kill 823c9944-fa2a-4c23-b921-3cc7398f0fc5 --signal=term
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				options.ExecID = args[0]
			}
			return options.Run()
		},
	}
	CmdWithArg(cmd, `EXEC_ID  The exec to signal.`)
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	FlagOutput(flags, &options.Output)
	FlagSort(flags, &options.Sort)
	FlagColor(flags, &options.Color)
	FlagObjectSelector(flags, &options.Selector)
	FlagRID(flags, &options.RID)
	flags.StringSliceVar(&options.Origins, "origin", nil, "signal the execs this submitted (api, imon, nmon, scheduler)")
	flags.StringVar(&options.SessionID, "session-id", "", "signal the execs of this session, which is the whole of one submitted command")
	flags.StringVar(&options.OrchestrationID, "orchestration-id", "", "signal the execs run under this orchestration")
	flags.StringVar(&options.Signal, "signal", "", "the signal to send, as a name (term, sigterm) or a number (15) (default \"kill\")")
	flags.BoolVar(&options.DryRun, "dry-run", false, "report what would be signaled, and signal nothing")
	return cmd
}

func (t *CmdDaemonKill) Run() error {
	// Refused here as well as in the daemon: a command that fans out over the
	// nodes must not reach any of them with nothing to narrow by.
	if t.ExecID == "" && t.SessionID == "" && t.OrchestrationID == "" &&
		len(t.Origins) == 0 && t.RID == "" && t.Selector == "" {
		return fmt.Errorf("name what to signal: an exec id, or one of --session-id, --orchestration-id, --rid, --origin, --service")
	}

	params := api.DeleteDaemonExecsParams{}
	if t.Signal != "" {
		params.Signal = &t.Signal
	}
	if t.DryRun {
		params.DryRun = &t.DryRun
	}
	if len(t.Origins) > 0 {
		params.Origins = &t.Origins
	}
	if t.RID != "" {
		params.RID = &t.RID
	}
	if t.Selector != "" {
		params.Selector = &t.Selector
	}
	for _, spec := range []struct {
		s    string
		name string
		p    **uuid.UUID
	}{
		{t.ExecID, "exec id", &params.ExecID},
		{t.SessionID, "session id", &params.SessionID},
		{t.OrchestrationID, "orchestration id", &params.OrchestrationID},
	} {
		if spec.s == "" {
			continue
		}
		id, err := uuid.Parse(spec.s)
		if err != nil {
			return fmt.Errorf("%s %s: %w", spec.name, spec.s, err)
		}
		*spec.p = &id
	}

	c, err := client.New()
	if err != nil {
		return err
	}
	if t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	if len(nodenames) == 0 {
		return fmt.Errorf("no node matching %s", t.NodeSelector)
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
			l, err := t.one(ctx, c, nodename, params)
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

	// What was signaled is reported even when a node could not be reached:
	// the signals that were sent were sent.
	errs = errors.Join(errs, output.Renderer{
		DefaultOutput: execPsColumns,
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          ToExecViews(items),
		Colorize:      rawconfig.Colorize,
	}.Print())
	return errs
}

func (t *CmdDaemonKill) one(ctx context.Context, c *client.T, nodename string, params api.DeleteDaemonExecsParams) ([]api.ExecItem, error) {
	resp, err := c.DeleteDaemonExecsWithResponse(ctx, nodename, &params)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", nodename, err)
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		return resp.JSON200.Items, nil
	case http.StatusBadRequest:
		return nil, fmt.Errorf("%s: %s", nodename, resp.JSON400)
	default:
		return nil, fmt.Errorf("%s: %s", nodename, resp.Status())
	}
}
