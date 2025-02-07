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
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/render/tree"
	"github.com/opensvc/om3/util/xsession"
)

type (
	// T has the same attributes as Action, but the interface
	// method implementation differ.
	T struct {
		actionrouter.T
		LocalFunc  func(context.Context, naming.Path) (any, error)
		RemoteFunc func(context.Context, naming.Path, string) (any, error)
	}

	asyncResult struct {
		Path            string    `json:"path"`
		OrchestrationID uuid.UUID `json:"orchestration_id,omitempty"`
		Error           error     `json:"error,omitempty"`
	}

	asyncResults []asyncResult
)

// New allocates a new client configuration and returns the reference
// so users are not tempted to use client.Config{} dereferenced, which would
// make loadContext useless.
func New(opts ...funcopt.O) *T {
	t := &T{}
	_ = funcopt.Apply(t, opts...)
	if t.NodeSelector != "" && t.DefaultOutput == "" {
		t.DefaultOutput = "tab=OBJECT:path,NODE:nodename,SID:data.session_id"
	}
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

// WithOutput controls the output data format.
// <empty>   => human readable format
// json      => json machine readable format
// flat      => flattened json (<k>=<v>) machine readable format
// flat_json => same as flat (backward compat)
func WithOutput(s string) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.Output = s
		return nil
	})
}

func WithDefaultOutput(s string) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.DefaultOutput = s
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
func WithLocalFunc(f func(context.Context, naming.Path) (any, error)) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.LocalFunc = f
		return nil
	})
}

// WithRemoteFunc sets a function to run if the action is local
func WithRemoteFunc(f func(context.Context, naming.Path, string) (any, error)) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.RemoteFunc = f
		return nil
	})
}

// Options returns the base Action struct
func (t T) Options() actionrouter.T {
	return t.T
}

func rsHumanRender(rs []actionrouter.Result) string {
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
			//		case (r.Error != nil) && fmt.Sprint(r.Error) != "":
			//			log.Error().Msgf("%s: %s", r.Path, r.Error)
		case r.Panic != nil:
			switch err := r.Panic.(type) {
			case error:
				log.Fatal().Stack().Msgf("%s: %s", r.Path, err)
			default:
				log.Fatal().Msgf("%s: %s", r.Path, err)
			}
		}
		if i, ok := r.Data.(treeProvider); ok {
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
				log.Error().Msgf("%s: unimplemented default renderer for local action result of type %s", r.Path, reflect.TypeOf(v))
			}
		}
	}
	if rsTree != nil {
		return rsTree.Render()
	}
	return s
}

func (t T) HasLocal() bool {
	return t.LocalFunc != nil
}

func (t T) DoLocal() error {
	if t.LocalFunc == nil {
		return fmt.Errorf("local mode is not available (use 'om' on a cluster node or 'ox --node ...')")
	}
	log.Debug().
		Str("format", t.Output).
		Str("selector", t.ObjectSelector).
		Msgf("do local object selection action")
	sel := objectselector.New(
		t.ObjectSelector,
		objectselector.WithLocal(true),
	)
	paths, err := sel.MustExpand()
	if err != nil {
		return err
	}
	paths = paths.Existing()
	if len(paths) == 0 {
		return fmt.Errorf("%s exists but has no local instance", t.ObjectSelector)
	}

	if t.Digest && isatty.IsTerminal(os.Stdin.Fd()) && (zerolog.GlobalLevel() != zerolog.DebugLevel) {
		fmt.Printf("sid=%s\n", xsession.ID)
	}
	var errs error
	results := make([]actionrouter.Result, 0)
	resultQ := make(chan actionrouter.Result)
	done := 0
	todo := len(paths)
	if todo == 0 {
		return nil
	}
	ctx := context.Background()
	ctx = actioncontext.WithRID(ctx, t.RID)
	ctx = actioncontext.WithTag(ctx, t.Tag)
	ctx = actioncontext.WithSubset(ctx, t.Subset)

	for _, path := range paths {
		t.instanceDo(ctx, resultQ, hostname.Hostname(), path, func(ctx context.Context, n string, p naming.Path) (any, error) {
			v, err := t.LocalFunc(ctx, p)
			if err != nil {
				return v, fmt.Errorf("%s: %w", p, err)
			}
			return v, nil
		})
	}
	for {
		result := <-resultQ
		results = append(results, result)
		if result.Error != nil {
			errs = errors.Join(errs, result.Error)
		}
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
		HumanRenderer: func() string { return rsHumanRender(results) },
		Colorize:      rawconfig.Colorize,
	}.Print()
	return errs
}

// DoAsync uses the agent API to submit a target state to reach via an
// orchestration.
func (t T) DoAsync() error {
	target, ok := instance.MonitorGlobalExpectValues[t.Target]
	if !ok {
		return fmt.Errorf("unexpected action: %s", t.Target)
	}
	c, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}
	sel := objectselector.New(
		t.ObjectSelector,
		objectselector.WithClient(c),
	)
	paths, err := sel.MustExpand()
	if err != nil {
		return err
	}
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
	rs := make(asyncResults, 0)
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
		switch target {
		case instance.MonitorGlobalExpectAborted:
			if resp, e := c.PostObjectActionAbortWithResponse(ctx, p.Namespace, p.Kind, p.Name); e != nil {
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
				case 404:
					err = fmt.Errorf("%s", resp.JSON404)
				case 408:
					err = fmt.Errorf("%s", resp.JSON408)
				case 409:
					err = fmt.Errorf("%s", resp.JSON409)
				case 500:
					err = fmt.Errorf("%s", resp.JSON500)
				}
			}
		case instance.MonitorGlobalExpectDeleted:
			if resp, e := c.PostObjectActionDeleteWithResponse(ctx, p.Namespace, p.Kind, p.Name); e != nil {
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
				case 404:
					err = fmt.Errorf("%s", resp.JSON404)
				case 408:
					err = fmt.Errorf("%s", resp.JSON408)
				case 409:
					err = fmt.Errorf("%s", resp.JSON409)
				case 500:
					err = fmt.Errorf("%s", resp.JSON500)
				}
			}
		case instance.MonitorGlobalExpectFrozen:
			if resp, e := c.PostObjectActionFreezeWithResponse(ctx, p.Namespace, p.Kind, p.Name); e != nil {
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
				case 404:
					err = fmt.Errorf("%s", resp.JSON404)
				case 408:
					err = fmt.Errorf("%s", resp.JSON408)
				case 409:
					err = fmt.Errorf("%s", resp.JSON409)
				case 500:
					err = fmt.Errorf("%s", resp.JSON500)
				}
			}
		case instance.MonitorGlobalExpectProvisioned:
			if resp, e := c.PostObjectActionProvisionWithResponse(ctx, p.Namespace, p.Kind, p.Name); e != nil {
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
				case 404:
					err = fmt.Errorf("%s", resp.JSON404)
				case 408:
					err = fmt.Errorf("%s", resp.JSON408)
				case 409:
					err = fmt.Errorf("%s", resp.JSON409)
				case 500:
					err = fmt.Errorf("%s", resp.JSON500)
				}
			}
		case instance.MonitorGlobalExpectPurged:
			if resp, e := c.PostObjectActionPurgeWithResponse(ctx, p.Namespace, p.Kind, p.Name); e != nil {
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
				case 404:
					err = fmt.Errorf("%s", resp.JSON404)
				case 408:
					err = fmt.Errorf("%s", resp.JSON408)
				case 409:
					err = fmt.Errorf("%s", resp.JSON409)
				case 500:
					err = fmt.Errorf("%s", resp.JSON500)
				}
			}
		case instance.MonitorGlobalExpectRestarted:
			params := api.PostObjectActionRestart{}
			if options, ok := t.TargetOptions.(instance.MonitorGlobalExpectOptionsRestarted); !ok {
				return fmt.Errorf("unexpected orchestration options: %#v", t.TargetOptions)
			} else {
				params.Force = &options.Force
			}

			if resp, e := c.PostObjectActionRestartWithResponse(ctx, p.Namespace, p.Kind, p.Name, params); e != nil {
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
				case 404:
					err = fmt.Errorf("%s", resp.JSON404)
				case 408:
					err = fmt.Errorf("%s", resp.JSON408)
				case 409:
					err = fmt.Errorf("%s", resp.JSON409)
				case 500:
					err = fmt.Errorf("%s", resp.JSON500)
				}
			}
		case instance.MonitorGlobalExpectStarted:
			if resp, e := c.PostObjectActionStartWithResponse(ctx, p.Namespace, p.Kind, p.Name); e != nil {
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
				case 404:
					err = fmt.Errorf("%s", resp.JSON404)
				case 408:
					err = fmt.Errorf("%s", resp.JSON408)
				case 409:
					err = fmt.Errorf("%s", resp.JSON409)
				case 500:
					err = fmt.Errorf("%s", resp.JSON500)
				}
			}
		case instance.MonitorGlobalExpectStopped:
			if resp, e := c.PostObjectActionStopWithResponse(ctx, p.Namespace, p.Kind, p.Name); e != nil {
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
				case 404:
					err = fmt.Errorf("%s", resp.JSON404)
				case 408:
					err = fmt.Errorf("%s", resp.JSON408)
				case 409:
					err = fmt.Errorf("%s", resp.JSON409)
				case 500:
					err = fmt.Errorf("%s", resp.JSON500)
				}
			}
		case instance.MonitorGlobalExpectUnfrozen:
			if resp, e := c.PostObjectActionUnfreezeWithResponse(ctx, p.Namespace, p.Kind, p.Name); e != nil {
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
				case 404:
					err = fmt.Errorf("%s", resp.JSON404)
				case 408:
					err = fmt.Errorf("%s", resp.JSON408)
				case 409:
					err = fmt.Errorf("%s", resp.JSON409)
				case 500:
					err = fmt.Errorf("%s", resp.JSON500)
				}
			}
		case instance.MonitorGlobalExpectUnprovisioned:
			if resp, e := c.PostObjectActionUnprovisionWithResponse(ctx, p.Namespace, p.Kind, p.Name); e != nil {
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
				case 404:
					err = fmt.Errorf("%s", resp.JSON404)
				case 408:
					err = fmt.Errorf("%s", resp.JSON408)
				case 409:
					err = fmt.Errorf("%s", resp.JSON409)
				case 500:
					err = fmt.Errorf("%s", resp.JSON500)
				}
			}
		case instance.MonitorGlobalExpectPlaced:
			if resp, e := c.PostObjectActionGivebackWithResponse(ctx, p.Namespace, p.Kind, p.Name); e != nil {
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
				case 404:
					err = fmt.Errorf("%s", resp.JSON404)
				case 408:
					err = fmt.Errorf("%s", resp.JSON408)
				case 409:
					err = fmt.Errorf("%s", resp.JSON409)
				case 500:
					err = fmt.Errorf("%s", resp.JSON500)
				}
			}
		case instance.MonitorGlobalExpectPlacedAt:
			params := api.PostObjectActionSwitch{}
			if options, ok := t.TargetOptions.(instance.MonitorGlobalExpectOptionsPlacedAt); !ok {
				return fmt.Errorf("unexpected orchestration options: %#v", t.TargetOptions)
			} else {
				params.Destination = options.Destination
			}
			if resp, e := c.PostObjectActionSwitchWithResponse(ctx, p.Namespace, p.Kind, p.Name, params); e != nil {
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
				case 404:
					err = fmt.Errorf("%s", resp.JSON404)
				case 408:
					err = fmt.Errorf("%s", resp.JSON408)
				case 409:
					err = fmt.Errorf("%s", resp.JSON409)
				case 500:
					err = fmt.Errorf("%s", resp.JSON500)
				}
			}
		default:
			return fmt.Errorf("unexpected global expect: %s", target)
		}
		var r asyncResult
		if err != nil {
			r = asyncResult{
				Error: err,
				Path:  p.String(),
			}
		} else {
			toWait++
			var orchestrationQueued api.OrchestrationQueued
			if err := json.Unmarshal(b, &orchestrationQueued); err == nil {
				r = asyncResult{
					OrchestrationID: orchestrationQueued.OrchestrationID,
					Path:            p.String(),
				}
			} else {
				r = asyncResult{
					Error: err,
					Path:  p.String(),
				}
			}
		}
		rs = append(rs, r)
	}
	output.Renderer{
		DefaultOutput: "tab=OBJECT:path,ORCHESTRATION_ID:orchestration_id,ERROR:error",
		Output:        t.Output,
		Color:         t.Color,
		Data:          rs,
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
					errs = errors.Join(errs, err)
				}
			}
		}
	}
	return errs
}

// DoRemote posts the action to a peer node agent API, for synchronous
// execution.
func (t T) DoRemote() error {
	if t.RemoteFunc == nil {
		return fmt.Errorf("no remote function defined")
	}

	c, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}
	params := api.GetObjectsParams{Path: &t.ObjectSelector}
	resp, err := c.GetObjectsWithResponse(context.Background(), &params)
	if err != nil {
		return fmt.Errorf("api: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected GET /objects status: %s", resp.Status())
	}

	nodenames := make(map[string]any)
	if l, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand(); err != nil {
		return err
	} else {
		for _, s := range l {
			nodenames[s] = nil
		}
	}

	results := make([]actionrouter.Result, 0)
	resultQ := make(chan actionrouter.Result)
	done := 0
	todo := 0
	requesterSid := xsession.ID

	var (
		cancel context.CancelFunc
		errs   error
		waitC  chan error
		count  int
	)

	ctx := context.Background()

	if t.WaitDuration > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), t.WaitDuration)
		defer cancel()
	} else {
		ctx, cancel = context.WithCancel(context.Background())
		defer cancel()
	}

	for i, item := range resp.JSON200.Items {
		selectedInstances := make(api.InstanceMap)
		for n, i := range item.Data.Instances {
			if _, ok := nodenames[n]; ok {
				selectedInstances[n] = i
			}
		}
		if t.Target == "started" && (string(item.Data.Topology) == topology.Failover.String()) && len(selectedInstances) > 1 {
			return fmt.Errorf("%s: cowardly refusing to start multiple instances: topology is failover", item.Meta.Object)
		}
		resp.JSON200.Items[i].Data.Instances = selectedInstances
		count += len(selectedInstances)
	}

	if t.Wait {
		waitC = make(chan error, count)
	}

	for _, item := range resp.JSON200.Items {
		for n := range item.Data.Instances {
			p, err := naming.ParsePath(item.Meta.Object)
			if err != nil {
				return err
			}
			if t.Wait {
				t.waitRequesterSessionEnd(ctx, c, requesterSid, p, waitC)
			}
			t.instanceDo(ctx, resultQ, n, p, func(ctx context.Context, n string, p naming.Path) (any, error) {
				return t.RemoteFunc(ctx, p, n)
			})
			todo++
		}
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
		HumanRenderer: func() string { return rsHumanRender(results) },
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

func (t T) Do() error {
	return actionrouter.Do(t)
}

// instanceDo executes the action in a goroutine
func (t T) instanceDo(ctx context.Context, resultQ chan actionrouter.Result, nodename string, path naming.Path, fn func(context.Context, string, naming.Path) (any, error)) {
	// push a progress view to the context, so objects can use it to
	// display what they are doing.

	go func(n string, p naming.Path) {
		result := actionrouter.Result{
			Path:     p,
			Nodename: n,
		}
		/*
			defer func() {
				if r := recover(); r != nil {
					result.Panic = r
					fmt.Println(string(debug.Stack()))
					resultQ <- result
				}
			}()
		*/
		data, err := fn(ctx, n, p)
		result.Data = data
		result.Error = err
		result.HumanRenderer = func() string { return actionrouter.DefaultHumanRenderer(data) }
		resultQ <- result
	}(nodename, path)
}

// waitExpectation will subscribe on path related messages, and will write to errC when expectation in not reached
// It starts new subscription before return to avoid missed events.
// it starts go routine to watch events for expectation reached
func (t T) waitExpectation(ctx context.Context, c *client.T, expectation string, p naming.Path, errC chan<- error) {
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
		filters = []string{"ObjectOrchestrationEnd,path=" + p.String()}
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
				err = fmt.Errorf("%s: can't wait expectation %s, got SetInstanceMonitorRefused", p, expectation)
				log.Debug().Msgf("%s", err)
				return
			case *msgbus.ObjectOrchestrationEnd:
				if instance.MonitorGlobalExpectValues[t.Target] == m.GlobalExpect {
					log.Debug().Msgf("%s: reached expectation %s (global expect was %s id: %s)", p, expectation, m.GlobalExpect, m.ID)
					return
				}
			case *msgbus.ObjectStatusDeleted:
				log.Debug().Msgf("%s: reached expectation %s (deleted)", p, expectation)
				return
			}
		}
	}()
}

func (t T) waitRequesterSessionEnd(ctx context.Context, c *client.T, requesterSid uuid.UUID, p naming.Path, errC chan<- error) {
	var (
		filters []string
		msg     pubsub.Messager

		err      error
		evReader event.ReadCloser
	)
	filters = []string{
		fmt.Sprintf("ObjectStatusDeleted,path=%s", p),
		fmt.Sprintf("ExecFailed,path=%s,.requester_session_id=%s", p, requesterSid),
		fmt.Sprintf("ExecSuccess,path=%s,.requester_session_id=%s", p, requesterSid),
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
				err = fmt.Errorf("wait requester session end failed on object %s: %w", p, err)
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
			case *msgbus.ExecSuccess:
				return
			case *msgbus.ExecFailed:
				err = errors.New(m.ErrS)
				return
			case *msgbus.ObjectStatusDeleted:
				log.Debug().Msgf("%s: stop waiting requester session id end (deleted)", p)
				return
			}
		}
	}()
}

func (t asyncResult) Unstructured() map[string]any {
	var errorString string
	if t.Error != nil {
		errorString = t.Error.Error()
	} else {
		errorString = "-"
	}
	return map[string]any{
		"orchestration_id": t.OrchestrationID.String(),
		"path":             t.Path,
		"error":            errorString,
	}
}
