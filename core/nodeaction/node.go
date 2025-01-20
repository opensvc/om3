package nodeaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/actionrouter"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/xsession"
)

type (
	// T has is an actionrouter.T with a node func
	T struct {
		actionrouter.T
		AsyncWaitNode string
		AsyncFunc     func(context.Context) error
		LocalFunc     func() (any, error)
		RemoteFunc    func(context.Context, string) (any, error)
	}

	Expectation any
)

func New(opts ...funcopt.O) *T {
	t := &T{}
	_ = funcopt.Apply(t, opts...)
	if t.NodeSelector != "" && t.DefaultOutput == "" {
		t.DefaultOutput = "tab=NODE:nodename,SID:data.session_id"
	}
	return t
}

// WithRemoteNodes expands into a selection of nodes to execute the
// action on.
func WithRemoteNodes(s string) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.NodeSelector = s
		return nil
	})
}

func WithAsyncWaitNode(s string) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.AsyncWaitNode = s
		return nil
	})
}

// WithAsyncFunc sets a function to run if the action is async
func WithAsyncFunc(f func(context.Context) error) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.AsyncFunc = f
		return nil
	})
}

// WithRemoteFunc sets a function to run if the action is local
func WithRemoteFunc(f func(context.Context, string) (any, error)) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.RemoteFunc = f
		return nil
	})
}

// WithLocal routes the action to the CRM instead of remoting it via
// orchestration or remote execution.
func WithLocal(v bool) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.Local = v
		return nil
	})
}

// LocalFirst makes actions not explicitly Local nor remoted
// via NodeSelector be treated as local (CRM level).
func LocalFirst() funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.DefaultIsLocal = true
		return nil
	})
}

// WithAsyncTarget is the node or object state the daemons should orchestrate
// to reach.
func WithAsyncTarget(s string) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.Target = s
		return nil
	})
}

// WithAsyncTime is the maximum duration to wait for an async action
// It needs WithAsyncWait(true)
func WithAsyncTime(d time.Duration) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.WaitDuration = d
		return nil
	})
}

// WithAsyncWait runs an event-watcher waiting for target state, global expect reached
func WithAsyncWait(v bool) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.Wait = v
		return nil
	})
}

// WithAsyncWatch runs an event-driven monitor on the selected objects after
// setting a new target. So the operator can see the orchestration
// unfolding.
func WithAsyncWatch(v bool) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.Watch = v
		return nil
	})
}

// WithFormat controls the output data format.
// <empty>   => human readable format
// json      => json machine readable format
// flat      => flattened json (<k>=<v>) machine readable format
// flat_json => same as flat (backward compat)
func WithFormat(s string) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.Output = s
		return nil
	})
}

// WithColor activates the colorization of outputs
// auto => yes if os.Stdout is a tty
// yes
// no
func WithColor(s string) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.Color = s
		return nil
	})
}

// WithLocalFunc sets a function to run if the action is local
func WithLocalFunc(f func() (any, error)) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.LocalFunc = f
		return nil
	})
}

// Options returns the base Action struct
func (t T) Options() actionrouter.T {
	return t.T
}

func human(r actionrouter.Result) string {
	if r.Error != nil {
		fmt.Fprintf(os.Stderr, "%s", r.Error)
	}
	if r.Panic != nil {
		fmt.Fprintf(os.Stderr, "%s", r.Panic)
	}
	s := ""
	if r.HumanRenderer != nil {
		s += r.HumanRenderer()
	} else if r.Data != nil {
		if b, err := yaml.Marshal(r.Data); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "%s", err)
		} else {
			_, _ = os.Stdout.Write(b)
		}
	}
	return s
}

func (t T) HasLocal() bool {
	return t.LocalFunc != nil
}

func (t T) DoLocal() error {
	results := make([]actionrouter.Result, 0)
	resultQ := make(chan actionrouter.Result)
	ctx := context.Background()
	if t.LocalFunc == nil {
		return fmt.Errorf("local node action is not implemented")
	}
	t.nodeDo(ctx, resultQ, hostname.Hostname(), func(_ context.Context, nodename string) (any, error) { return t.LocalFunc() })
	result := <-resultQ
	results = append(results, result)
	if result.Error != nil {
		return result.Error
	}
	output.Renderer{
		Output:        t.Output,
		Color:         t.Color,
		Data:          []actionrouter.Result{result},
		Colorize:      rawconfig.Colorize,
		HumanRenderer: func() string { return human(result) },
	}.Print()
	return nil
}

// DoAsync uses the agent API to submit a target state to reach via an
// orchestration.
func (t T) DoAsync() error {
	var (
		ctx         context.Context
		cancel      context.CancelFunc
		expectation any
		waitC       = make(chan error)
		b           []byte
	)
	c, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}
	if t.WaitDuration > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), t.WaitDuration)
		defer cancel()
	} else {
		ctx, cancel = context.WithCancel(context.Background())
		defer cancel()
	}
	if t.Wait {
		switch t.Target {
		case node.MonitorStateDrained.String():
			expectation = node.MonitorStateDrained
		case node.MonitorGlobalExpectAborted.String():
			expectation = node.MonitorGlobalExpectAborted
		case node.MonitorGlobalExpectFrozen.String():
			expectation = node.MonitorGlobalExpectFrozen
		case node.MonitorGlobalExpectThawed.String():
			expectation = node.MonitorGlobalExpectThawed
		default:
			return fmt.Errorf("unexpected target: %s", t.Target)
		}
		t.waitExpectation(ctx, c, expectation, waitC)
	}
	if t.AsyncFunc != nil {
		if err := t.AsyncFunc(ctx); err != nil {
			return err
		}
	} else {
		switch t.Target {
		case node.MonitorGlobalExpectAborted.String():
			expectation = node.MonitorGlobalExpectAborted
			if resp, e := c.PostClusterActionAbortWithResponse(ctx); e != nil {
				err = e
			} else {
				switch resp.StatusCode() {
				case http.StatusOK:
					b = resp.Body
				case 400:
					err = fmt.Errorf("%s", resp.JSON400)
				case 401:
					err = fmt.Errorf("%s", resp.JSON401)
				case 403:
					err = fmt.Errorf("%s", resp.JSON403)
				case 408:
					err = fmt.Errorf("%s", resp.JSON408)
				case 409:
					err = fmt.Errorf("%s", resp.JSON409)
				case 500:
					err = fmt.Errorf("%s", resp.JSON500)
				}
			}
		case node.MonitorGlobalExpectFrozen.String():
			expectation = node.MonitorGlobalExpectFrozen
			if resp, e := c.PostClusterActionFreezeWithResponse(ctx); e != nil {
				err = e
			} else {
				switch resp.StatusCode() {
				case http.StatusOK:
					b = resp.Body
				case 400:
					err = fmt.Errorf("%s", resp.JSON400)
				case 401:
					err = fmt.Errorf("%s", resp.JSON401)
				case 403:
					err = fmt.Errorf("%s", resp.JSON403)
				case 408:
					err = fmt.Errorf("%s", resp.JSON408)
				case 409:
					err = fmt.Errorf("%s", resp.JSON409)
				case 500:
					err = fmt.Errorf("%s", resp.JSON500)
				}
			}
		case node.MonitorGlobalExpectThawed.String():
			expectation = node.MonitorGlobalExpectThawed
			if resp, e := c.PostClusterActionUnfreezeWithResponse(ctx); e != nil {
				err = e
			} else {
				switch resp.StatusCode() {
				case http.StatusOK:
					b = resp.Body
				case 400:
					err = fmt.Errorf("%s", resp.JSON400)
				case 401:
					err = fmt.Errorf("%s", resp.JSON401)
				case 403:
					err = fmt.Errorf("%s", resp.JSON403)
				case 408:
					err = fmt.Errorf("%s", resp.JSON408)
				case 409:
					err = fmt.Errorf("%s", resp.JSON409)
				case 500:
					err = fmt.Errorf("%s", resp.JSON500)
				}
			}
		default:
			return fmt.Errorf("unexpected target: %s", t.Target)
		}

		if err == nil {
			var orchestrationQueued api.OrchestrationQueued
			if err := json.Unmarshal(b, &orchestrationQueued); err == nil {
				fmt.Println(orchestrationQueued.OrchestrationID)
			} else {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}

	if t.Wait {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-waitC:
			return err
		}
	}

	return err
}

// DoRemote posts the action to a peer node agent API, for synchronous
// execution.
func (t T) DoRemote() error {
	c, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}

	results := make([]actionrouter.Result, 0)
	resultQ := make(chan actionrouter.Result)
	count := len(nodenames)
	done := 0
	todo := 0
	requesterSid := xsession.ID

	var (
		cancel context.CancelFunc
		errs   error
		waitC  chan error
	)

	ctx := context.Background()

	if t.WaitDuration > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), t.WaitDuration)
		defer cancel()
	} else {
		ctx, cancel = context.WithCancel(context.Background())
		defer cancel()
	}

	if t.Wait {
		waitC = make(chan error, count)
	}

	for _, nodename := range nodenames {
		if t.Wait {
			t.waitRequesterSessionEnd(ctx, c, nodename, requesterSid, waitC)
		}
		if t.RemoteFunc == nil {
			return fmt.Errorf("RemoteFunc is nil")
		}
		t.nodeDo(ctx, resultQ, nodename, func(ctx context.Context, nodename string) (any, error) {
			return t.RemoteFunc(ctx, nodename)
		})
		todo++
	}
	if todo == 0 {
		return nil
	}
	for {
		result := <-resultQ
		switch {
		case result.Panic != nil:
			fmt.Fprintln(os.Stderr, result.Panic)
			errs = errors.New("remote action error")
		case result.Error != nil:
			fmt.Fprintln(os.Stderr, result.Error)
			errs = errors.New("remote action error")
		}
		results = append(results, result)
		done++
		if done >= todo {
			break
		}
	}
	output.Renderer{
		DefaultOutput: t.DefaultOutput,
		Output:        t.Output,
		Color:         t.Color,
		Data:          results,
		Colorize:      rawconfig.Colorize,
	}.Print()
	if t.Wait && todo > 0 {
		for i := 0; i < todo; i++ {
			select {
			case <-ctx.Done():
				errs = errors.Join(errs, ctx.Err())
				return errs
			case err := <-waitC:
				if err != nil {
					errs = errors.Join(errs, err)
				}
			}
		}
	}
	return errs
}

func (t T) waitRequesterSessionEnd(ctx context.Context, c *client.T, nodename string, requesterSid uuid.UUID, errC chan<- error) {
	var (
		filters []string
		msg     pubsub.Messager

		err      error
		evReader event.ReadCloser
	)
	filters = []string{
		fmt.Sprintf("NodeMonitorDeleted"),
		fmt.Sprintf("ExecFailed,requester_sid=%s", requesterSid),
		fmt.Sprintf("ExecSuccess,requester_sid=%s", requesterSid),
	}
	getEvents := c.NewGetEvents().SetFilters(filters)
	if t.WaitDuration > 0 {
		getEvents = getEvents.SetDuration(t.WaitDuration)
	}
	evReader, err = getEvents.GetReader()
	if err != nil {
		return
	}

	if x, ok := evReader.(event.ContextSetter); ok {
		x.SetContext(ctx)
	}
	go func() {
		defer func() {
			if err != nil {
				err = fmt.Errorf("wait requester session end failed on node %s: %w", nodename, err)
			}
			select {
			case <-ctx.Done():
			case errC <- err:
			}
		}()

		go func() {
			// close reader when ctx is done
			select {
			case <-ctx.Done():
				_ = evReader.Close()
			}
		}()
		for {
			ev, readError := evReader.Read()
			if readError != nil {
				if errors.Is(readError, io.EOF) {
					err = fmt.Errorf("no more events, wait %s failed: %w", nodename, err)
				} else {
					err = readError
				}
				return
			}
			msg, err = msgbus.EventToMessage(*ev)
			if err != nil {
				return
			}
			switch m := msg.(type) {
			case *msgbus.ExecSuccess:
				return
			case *msgbus.ExecFailed:
				err = errors.New(m.ErrS)
				return
			case *msgbus.ObjectStatusDeleted:
				log.Debug().Msgf("%s: stop waiting requester session id end (deleted)", nodename)
				return
			}
		}
	}()
}

func (t T) Do() error {
	return actionrouter.Do(t)
}

// waitExpectation subscribes to NodeMonitorUpdated and wait for expectation reached
// It writes result to errC channel
func (t T) waitExpectation(ctx context.Context, c *client.T, exp Expectation, errC chan<- error) {
	var (
		filters      []string
		msg          msgbus.NodeMonitorUpdated
		reached      = make(map[string]bool)
		reachedUnset = make(map[string]bool)

		err      error
		evReader event.ReadCloser
	)
	log := plog.NewDefaultLogger().WithPrefix(fmt.Sprintf("nodeaction: wait %s: ", exp))
	switch exp.(type) {
	case node.MonitorState:
		filters = []string{"NodeMonitorUpdated,node=" + t.AsyncWaitNode}
	case node.MonitorGlobalExpect:
		filters = []string{"NodeMonitorUpdated"}
	}
	log.Debugf("get event with filters: %+v", filters)
	getEvents := c.NewGetEvents().SetFilters(filters)
	if t.WaitDuration > 0 {
		getEvents = getEvents.SetDuration(t.WaitDuration)
	}
	evReader, err = getEvents.GetReader()
	if err != nil {
		errC <- err
		return
	}

	if x, ok := evReader.(event.ContextSetter); ok {
		x.SetContext(ctx)
	}
	go func() {
		defer func() {
			if err != nil {
				err = fmt.Errorf("wait expectation %s failed: %w", exp, err)
			}
			select {
			case <-ctx.Done():
			case errC <- err:
			}
		}()

		go func() {
			// close reader when ctx is done
			select {
			case <-ctx.Done():
				_ = evReader.Close()
			}
		}()
		for {
			ev, readError := evReader.Read()
			if readError != nil {
				if errors.Is(readError, io.EOF) {
					err = fmt.Errorf("no more events (%w), wait %v failed", err, exp)
				} else {
					err = readError
				}
				return
			}
			switch ev.Kind {
			case "NodeMonitorUpdated":
				err = json.Unmarshal(ev.Data, &msg)
				if err != nil {
					return
				}
				log.Debugf("NodeMonitorUpdated %+v", msg)
				nmon := msg.Value
				switch v := exp.(type) {
				case node.MonitorState:
					if nmon.State == v {
						reached[msg.Node] = true
						log.Debugf("NodeMonitorUpdated reached state %s", v)
					} else if reached[msg.Node] && nmon.State == node.MonitorStateIdle {
						log.Debugf("NodeMonitorUpdated reached state %s unset", v)
						return
					}
				case node.MonitorGlobalExpect:
					if nmon.GlobalExpect == v {
						reached[msg.Node] = true
						log.Debugf("NodeMonitorUpdated reached global expect %s", v)
					} else if reached[msg.Node] && nmon.GlobalExpect == node.MonitorGlobalExpectNone {
						reachedUnset[msg.Node] = true
						log.Debugf("NodeMonitorUpdated reached global expect %s unset for %s", v, msg.Node)
					}
					if len(reached) > 0 && len(reached) == len(reachedUnset) {
						log.Debugf("NodeMonitorUpdated reached global expect %s unset for all nodes", v)
						return
					}
				}
			}
		}
	}()
}

func (t T) nodeDo(ctx context.Context, resultQ chan actionrouter.Result, nodename string, fn func(context.Context, string) (any, error)) {
	go func(nodename string) {
		result := actionrouter.Result{
			Nodename: nodename,
		}
		data, err := fn(ctx, nodename)
		result.Data = data
		result.Error = err
		result.HumanRenderer = func() string { return actionrouter.DefaultHumanRenderer(data) }
		resultQ <- result
	}(nodename)
}
