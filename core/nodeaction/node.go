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

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"

	"github.com/opensvc/om3/core/actionrouter"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
)

type (
	// T has is an actionrouter.T with a node func
	T struct {
		actionrouter.T
		Func func() (any, error)
	}

	Expectation any
)

func New(opts ...funcopt.O) *T {
	t := &T{}
	_ = funcopt.Apply(t, opts...)
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
func WithLocalRun(f func() (any, error)) funcopt.O {
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
	r := nodeDo(t.Func)
	human := func() string {
		if r.Error != nil {
			log.Error().Msgf("%s", r.Error)
		}
		if r.Panic != nil {
			log.Fatal().Msgf("%s", r.Panic)
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
	output.Renderer{
		Format:        t.Format,
		Color:         t.Color,
		Data:          []actionrouter.Result{r},
		HumanRenderer: human,
		Colorize:      rawconfig.Colorize,
	}.Print()
	var errs error
	if r.Panic != nil {
		errs = errors.Join(errs, fmt.Errorf("%s", r.Panic))
	}
	if r.Error != nil {
		errs = errors.Join(errs, r.Error)
	}
	return errs
}

// DoAsync uses the agent API to submit a target state to reach via an
// orchestration.
func (t T) DoAsync() error {
	var (
		ctx         context.Context
		cancel      context.CancelFunc
		expectation any
		waitC       = make(chan error)
	)
	c, err := client.New(client.WithURL(t.Server), client.WithTimeout(0))
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
	params := api.PostNodeMonitor{}
	switch t.Target {
	case "":
	case node.MonitorStateDrained.String():
		s := t.Target
		params.LocalExpect = &s
		expectation = node.MonitorStateDrained
	default:
		if globalExpect, ok := node.MonitorGlobalExpectValues[t.Target]; ok {
			s := t.Target
			params.GlobalExpect = &s
			expectation = globalExpect
		} else {
			return fmt.Errorf("unexpected global expect value %s", t.Target)
		}
	}
	if t.Wait {
		go t.waitExpectation(ctx, c, expectation, waitC)
	}
	resp, err := c.PostNodeMonitorWithResponse(ctx, params)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		human := func() string {
			s := fmt.Sprintln(resp.JSON200)
			return s
		}
		output.Renderer{
			Format:        t.Format,
			Color:         t.Color,
			Data:          resp.JSON200,
			HumanRenderer: human,
			Colorize:      rawconfig.Colorize,
		}.Print()
	case 400:
		return fmt.Errorf("%s", resp.JSON400)
	case 401:
		return fmt.Errorf("%s", resp.JSON403)
	case 403:
		return fmt.Errorf("%s", resp.JSON401)
	case 500:
		return fmt.Errorf("%s", resp.JSON500)
	default:
		return fmt.Errorf("unexpected status code %s", resp.Status())
	}

	if t.Wait {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-waitC:
			return err
		}
	}
	return nil
}

// DoRemote posts the action to a peer node agent API, for synchronous
// execution.
func (t T) DoRemote() error {
	/*
		c, err := client.New(client.WithURL(t.Server))
		if err != nil {
			return err
		}
		req := c.PostNodeAction()
		req.NodeSelector = t.NodeSelector
		req.Action = t.Action
		req.Options = t.PostFlags
		b, err := req.Do()
		if err != nil {
			return err
		}
		data := &struct {
			Err    string `json:"err" yaml:"err"`
			Out    string `json:"out" yaml:"out"`
			Status int    `json:"status" yaml:"status"`
		}{}
		if err := json.Unmarshal(b, data); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(os.Stdout, data.Out)
		_, _ = fmt.Fprintf(os.Stderr, data.Err)
	*/
	return fmt.Errorf("todo")
}

func (t T) Do() error {
	return actionrouter.Do(t)
}

func nodeDo(fn func() (any, error)) actionrouter.Result {
	data, err := fn()
	result := actionrouter.Result{
		Nodename:      hostname.Hostname(),
		HumanRenderer: func() string { return actionrouter.DefaultHumanRenderer(data) },
	}
	result.Data = data
	result.Error = err
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("do")
	}
	return result
}

// waitExpectation subscribes to NodeMonitorUpdated and wait for expectation reached
// It writes result to errC chanel
func (t T) waitExpectation(ctx context.Context, c *client.T, exp Expectation, errC chan<- error) {
	var (
		filters      []string
		msg          msgbus.NodeMonitorUpdated
		reached      = make(map[string]bool)
		reachedUnset = make(map[string]bool)

		err      error
		evReader event.ReadCloser
	)
	defer func() {
		select {
		case <-ctx.Done():
		case errC <- err:
		}
	}()
	switch exp.(type) {
	case node.MonitorState:
		filters = []string{"NodeMonitorUpdated,node=" + hostname.Hostname()}
	case node.MonitorGlobalExpect:
		filters = []string{"NodeMonitorUpdated"}
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
			log.Debug().Msgf("NodeMonitorUpdated %+v", msg)
			nmon := msg.Value
			switch v := exp.(type) {
			case node.MonitorState:
				if nmon.State == v {
					reached[msg.Node] = true
					log.Debug().Msgf("NodeMonitorUpdated reached state %s", v)
				} else if reached[msg.Node] && nmon.State == node.MonitorStateIdle {
					log.Debug().Msgf("NodeMonitorUpdated reached state %s unset", v)
					return
				}
			case node.MonitorGlobalExpect:
				if nmon.GlobalExpect == v {
					reached[msg.Node] = true
					log.Debug().Msgf("NodeMonitorUpdated reached global expect %s", v)
				} else if reached[msg.Node] && nmon.GlobalExpect == node.MonitorGlobalExpectNone {
					reachedUnset[msg.Node] = true
					log.Debug().Msgf("NodeMonitorUpdated reached global expect %s unset for %s", v, msg.Node)
				}
				if len(reached) > 0 && len(reached) == len(reachedUnset) {
					log.Debug().Msgf("NodeMonitorUpdated reached global expect %s unset for all nodes", v)
					return
				}
			}
		}
	}
}
