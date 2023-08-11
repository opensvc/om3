package objectaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/actionrouter"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/progress"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/render/tree"
	"github.com/opensvc/om3/util/xsession"
)

type (
	// T has the same attributes as Action, but the interface
	// method implementation differ.
	T struct {
		actionrouter.T
		Func func(context.Context, path.T) (any, error)
	}
)

// New allocates a new client configuration and returns the reference
// so users are not tempted to use client.Config{} dereferenced, which would
// make loadContext useless.
func New(opts ...funcopt.O) *T {
	t := &T{}
	_ = funcopt.Apply(t, opts...)
	return t
}

// WithObjectSelector expands into a selection of objects to execute
// the action on.
func WithObjectSelector(s string) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.ObjectSelector = s
		return nil
	})
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

// WithRID expands into a selection of resources to execute the action on.
func WithRID(s string) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.RID = s
		return nil
	})
}

// WithTag expands into a selection of resources to execute the action on.
func WithTag(s string) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.Tag = s
		return nil
	})
}

// WithSubset expands into a selection of resources to execute the action on.
func WithSubset(s string) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.Subset = s
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

// LocalFirst makes actions not explicitely Local nor remoted
// via NodeSelector be treated as local (CRM level).
func LocalFirst() funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.DefaultIsLocal = true
		return nil
	})
}

// WithRemoteAction is the name of the action as passed to the command line
// interface.
func WithRemoteAction(s string) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.Action = s
		return nil
	})
}

// WithRemoteOptions is the dataset submited in the POST /{object|node}_action
// api handler to execute the action remotely.
func WithRemoteOptions(m map[string]any) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.PostFlags = m
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

// WithAsyncTargetOptions is the options of the target defined by WithAsyncTarget.
func WithAsyncTargetOptions(o any) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.TargetOptions = o
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

// WithAsyncWait runs an event-watcher waiting for target state, global expect return to none
func WithAsyncWait(v bool) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.Wait = v
		return nil
	})
}

// WithProgress decides if the action progress renderer is used.
func WithProgress(v bool) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.WithProgress = v
		return nil
	})
}

// WithAsyncWatch runs a event-driven monitor on the selected objects after
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
		t.Format = s
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

// WithServer sets the api url.
func WithServer(s string) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.Server = s
		return nil
	})
}

// WithLocalRun sets a function to run if the action is local
func WithLocalRun(f func(context.Context, path.T) (any, error)) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.Func = f
		return nil
	})
}

// Options returns the base Action struct
func (t T) Options() actionrouter.T {
	return t.T
}

func (t T) DoLocal() error {
	log.Debug().
		Str("format", t.Format).
		Str("selector", t.ObjectSelector).
		Msg("do local object selection action")
	sel := objectselector.NewSelection(
		t.ObjectSelector,
		objectselector.SelectionWithLocal(true),
	)
	if t.Digest && isatty.IsTerminal(os.Stdin.Fd()) && (zerolog.GlobalLevel() != zerolog.DebugLevel) {
		fmt.Printf("sid=%s\n", xsession.ID)
	}
	rs, err := t.selectionDo(sel, t.Func)
	if err != nil {
		return err
	}
	human := func() string {
		var (
			rsTree *tree.Tree
			rsNode *tree.Node
		)
		type treeProvider interface {
			Tree() *tree.Tree
		}
		s := ""
		manyResults := len(rs) > 1
		for i, r := range rs {
			switch {
			case errors.Is(r.Error, object.ErrDisabled):
				if manyResults {
					fmt.Printf("%s: %s\n", r.Path, r.Error)
				} else {
					fmt.Printf("%s\n", r.Error)
				}
				rs[i].Error = nil
			case (r.Error != nil) && fmt.Sprint(r.Error) != "":
				log.Error().Err(r.Error).Send()
			case r.Panic != nil:
				switch err := r.Panic.(type) {
				case error:
					log.Fatal().Stack().Err(err).Send()
				default:
					log.Fatal().Msgf("%s", err)
				}
			}
			if i, ok := any(r.Data).(treeProvider); ok {
				if rsTree == nil {
					rsTree = tree.New()
				}
				branch := i.Tree()
				if !branch.IsEmpty() {
					rsNode = rsTree.AddNode()
					rsNode.AddColumn().AddText(r.Path.String() + " @ " + r.Nodename).SetColor(rawconfig.Color.Bold)
					rsNode.PlugTree(branch)
				}
				continue
			}
			switch {
			case r.HumanRenderer != nil:
				s += r.HumanRenderer()
			case r.Data != nil:
				switch v := r.Data.(type) {
				case string:
					s += fmt.Sprintln(v)
				case []string:
					for _, e := range v {
						s += fmt.Sprintln(e)
					}
				default:
					log.Error().Msgf("unimplemented default renderer for local action result of type %s", reflect.TypeOf(v))
				}
			}
		}
		if rsTree != nil {
			return rsTree.Render()
		}
		return s
	}
	output.Renderer{
		Format:        t.Format,
		Color:         t.Color,
		Data:          rs,
		HumanRenderer: human,
		Colorize:      rawconfig.Colorize,
	}.Print()
	var errs error
	for _, ar := range rs {
		switch {
		case ar.Panic != nil:
			errs = errors.Join(errs, fmt.Errorf("%s: %s", ar.Path, ar.Panic))
		case ar.Error != nil:
			errs = errors.Join(errs, fmt.Errorf("%s: %w", ar.Path, ar.Error))
		}
	}
	return errs
}

// DoAsync uses the agent API to submit a target state to reach via an
// orchestration.
func (t T) DoAsync() error {
	c, err := client.New(client.WithURL(t.Server), client.WithTimeout(0))
	if err != nil {
		return err
	}
	sel := objectselector.NewSelection(
		t.ObjectSelector,
		objectselector.SelectionWithClient(c),
	)
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	type (
		result struct {
			Path            string    `json:"path" yaml:"path"`
			OrchestrationId uuid.UUID `json:"orchestration_id,omitempty" yaml:"orchestration_id,omitempty"`
			Error           error     `json:"error,omitempty" yaml:"error,omitempty"`
		}
		results []result
	)
	var (
		ctx    context.Context
		cancel context.CancelFunc
		errs   error
		waitC  chan error
		toWait int
	)
	if t.WaitDuration > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), t.WaitDuration)
		defer cancel()
	} else {
		ctx, cancel = context.WithCancel(context.Background())
		defer cancel()
	}
	rs := make(results, 0)
	if t.Wait {
		waitC = make(chan error, len(paths))
	}
	for _, p := range paths {
		var (
			err error
			b   []byte
		)
		if t.Wait {
			t.waitExpectation(ctx, c, t.Target, p, waitC)
		}
		switch t.Target {
		case instance.MonitorGlobalExpectPlacedAt.String():
			params := api.PostObjectSwitchTo{}
			params.Path = p.String()
			options := t.TargetOptions.(instance.MonitorGlobalExpectOptionsPlacedAt)
			params.Destination = options.Destination
			resp, e := c.PostObjectSwitchToWithResponse(ctx, params)
			if e != nil {
				err = e
			}
			switch resp.StatusCode() {
			case http.StatusOK:
				b = resp.Body
			case 400:
				err = fmt.Errorf("%s", resp.JSON400)
			case 401:
				err = fmt.Errorf("%s", resp.JSON401)
			case 403:
				err = fmt.Errorf("%s", resp.JSON403)
			case 409:
				err = fmt.Errorf("%s", resp.JSON409)
			case 500:
				err = fmt.Errorf("%s", resp.JSON500)
			}

		default:
			params := api.PostObjectMonitor{}
			params.Path = p.String()
			params.GlobalExpect = &t.Target
			resp, e := c.PostObjectMonitorWithResponse(ctx, params)
			if e != nil {
				err = e
			}
			switch resp.StatusCode() {
			case http.StatusOK:
				b = resp.Body
			case 400:
				err = fmt.Errorf("%s", resp.JSON400)
			case 401:
				err = fmt.Errorf("%s", resp.JSON401)
			case 403:
				err = fmt.Errorf("%s", resp.JSON403)
			case 409:
				err = fmt.Errorf("%s", resp.JSON409)
			case 500:
				err = fmt.Errorf("%s", resp.JSON500)
			}
		}
		var r result
		if err != nil {
			r = result{
				Error: err,
				Path:  p.String(),
			}
		} else {
			toWait++
			var monitorUpdateQueued api.MonitorUpdateQueued
			if err := json.Unmarshal(b, &monitorUpdateQueued); err == nil {
				r = result{
					OrchestrationId: monitorUpdateQueued.OrchestrationId,
					Path:            p.String(),
				}
			} else {
				r = result{
					Error: err,
					Path:  p.String(),
				}
			}
		}
		rs = append(rs, r)
	}
	human := func() string {
		s := ""
		for _, r := range rs {
			if r.Error != nil {
				s += fmt.Sprintf("%s %s %s\n", r.OrchestrationId, r.Path, rawconfig.Colorize.Error(r.Error))
			} else {
				s += fmt.Sprintf("%s %s\n", r.OrchestrationId, r.Path)
			}
		}
		return s
	}
	output.Renderer{
		Format:        t.Format,
		Color:         t.Color,
		Data:          rs,
		HumanRenderer: human,
		Colorize:      rawconfig.Colorize,
	}.Print()
	if t.Wait && toWait > 0 {
		for i := 0; i < toWait; i++ {
			select {
			case <-ctx.Done():
				errs = errors.Join(errs, ctx.Err())
				return errs
			case err := <-waitC:
				if err != nil {
					errs = errors.Join(errs, ctx.Err())
				}
			}
		}
	}
	return errs
}

// DoRemote posts the action to a peer node agent API, for synchronous
// execution.
func (t T) DoRemote() error {
	/*
		c, err := client.New(client.WithURL(t.Server))
		if err != nil {
			return err
		}
		req := c.NewPostObjectAction()
		req.ObjectSelector = t.ObjectSelector
		req.NodeSelector = t.NodeSelector
		req.Action = t.Action
		req.Options = t.PostFlags
		b, err := req.Do()
		if err != nil {
			return err
		}
		human := func() string {
			s := fmt.Sprintln(string(b))
			return s
		}
		output.Renderer{
			Format:        t.Format,
			Color:         t.Color,
			Data:          b,
			HumanRenderer: human,
		}.Print()
	*/
	return fmt.Errorf("todo")
}

func (t T) Do() error {
	return actionrouter.Do(t)
}

// selectionDo executes in parallel the action on all selected objects supporting
// the action.
func (t T) selectionDo(selection *objectselector.Selection, fn func(context.Context, path.T) (any, error)) ([]actionrouter.Result, error) {
	results := make([]actionrouter.Result, 0)

	paths, err := selection.Expand()
	if err != nil {
		return results, err
	}

	ctx := context.Background()
	ctx = actioncontext.WithRID(ctx, t.RID)
	ctx = actioncontext.WithTag(ctx, t.Tag)
	ctx = actioncontext.WithSubset(ctx, t.Subset)

	// push a progress view to the context, so objects can use it to
	// display what they are doing.
	if t.WithProgress {
		progressView := progress.NewView()
		progressView.Start()
		defer progressView.Stop()
		ctx = progress.ContextWithView(ctx, progressView)
	}

	q := make(chan actionrouter.Result, len(paths))
	started := 0

	for _, p := range paths {
		go func(p path.T) {
			result := actionrouter.Result{
				Path:     p,
				Nodename: hostname.Hostname(),
			}
			defer func() {
				if r := recover(); r != nil {
					result.Panic = r
					fmt.Println(string(debug.Stack()))
					q <- result
				}
			}()
			data, err := fn(ctx, p)
			result.Data = data
			result.Error = err
			result.HumanRenderer = func() string { return actionrouter.DefaultHumanRenderer(data) }
			q <- result
		}(p)
		started++
	}

	for i := 0; i < started; i++ {
		r := <-q
		results = append(results, r)
	}
	return results, nil
}

// waitExpectation will subscribe on path related messages, and will write to errC when expectation in not reached
// It starts new subscription before return to avoid missed events.
// it starts go routine to watch events for expectation reached
func (t T) waitExpectation(ctx context.Context, c *client.T, expectation string, p path.T, errC chan<- error) {
	var (
		filters []string
		msg     pubsub.Messager

		err      error
		evReader event.ReadCloser
	)
	switch expectation {
	case instance.MonitorGlobalExpectPurged.String():
		filters = []string{"ObjectStatusDeleted,path=" + p.String()}
	default:
		filters = []string{"InstanceMonitorUpdated,path=" + p.String()}
	}
	filters = append(filters, "SetInstanceMonitorRefused,path="+p.String())
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
				err = fmt.Errorf("wait expectation %s failed on object %s: %w", expectation, p, err)
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
					err = fmt.Errorf("no more events, wait %v failed: %w", p, err)
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
			case *msgbus.SetInstanceMonitorRefused:
				err = fmt.Errorf("can't wait %s expectation %s, got SetInstanceMonitorRefused", p, expectation)
				log.Debug().Err(err).Msgf("waitExpectation")
				return
			case *msgbus.InstanceMonitorUpdated:
				if m.Value.GlobalExpect == instance.MonitorGlobalExpectNone {
					log.Debug().Msgf("InstanceMonitorUpdated %s reached global expect %s -> %s", p, expectation, m.Value.GlobalExpect)
					return
				}
			case *msgbus.ObjectStatusDeleted:
				log.Debug().Msgf("ObjectStatusDeleted %s reached %s", p, expectation)
				return
			}
		}
	}()
}
