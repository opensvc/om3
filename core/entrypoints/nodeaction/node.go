package nodeaction

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/entrypoints/action"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
)

type (
	// T has the same attributes as Action, but the interface
	// method implementation differ.
	T struct {
		action.T
		Node object.NodeAction
	}

	// Option is a functional option configurer.
	// https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis
	Option interface {
		apply(t *T) error
	}

	optionFunc func(*T) error
)

func (fn optionFunc) apply(t *T) error {
	return fn(t)
}

func New(opts ...Option) *T {
	t := &T{}
	for _, opt := range opts {
		_ = opt.apply(t)
	}
	return t
}

//
// WithNodeSelector expands into a selection of nodes to execute the
// action on.
//
func WithRemoteNodes(s string) Option {
	return optionFunc(func(t *T) error {
		t.NodeSelector = s
		return nil
	})
}

//
// WithLocal routes the action to the CRM instead of remoting it via
// orchestration or remote execution.
//
func WithLocal(v bool) Option {
	return optionFunc(func(t *T) error {
		t.Local = v
		return nil
	})
}

//
// LocalFirst makes actions not explicitely Local nor remoted
// via NodeSelector be treated as local (CRM level).
//
func LocalFirst() Option {
	return optionFunc(func(t *T) error {
		t.DefaultIsLocal = true
		return nil
	})
}

//
// WithRemoteAction is the name of the action as passed to the command line
// interface.
//
func WithRemoteAction(s string) Option {
	return optionFunc(func(t *T) error {
		t.Action = s
		return nil
	})
}

//
// WithRemoteOptions is the dataset submited in the POST /{object|node}_action
// api handler to execute the action remotely.
//
func WithRemoteOptions(m map[string]interface{}) Option {
	return optionFunc(func(t *T) error {
		t.PostFlags = m
		return nil
	})
}

//
// WithAsyncTarget is the node or object state the daemons should orchestrate
// to reach.
//
func WithAsyncTarget(s string) Option {
	return optionFunc(func(t *T) error {
		t.Target = s
		return nil
	})
}

//
// WithAsyncWatch runs a event-driven monitor on the selected objects after
// setting a new target. So the operator can see the orchestration
// unfolding.
//
func WithAsyncWatch(v bool) Option {
	return optionFunc(func(t *T) error {
		t.Watch = v
		return nil
	})
}

//
// WithFormat controls the output data format.
// <empty>   => human readable format
// json      => json machine readable format
// flat      => flattened json (<k>=<v>) machine readable format
// flat_json => same as flat (backward compat)
//
func WithFormat(s string) Option {
	return optionFunc(func(t *T) error {
		t.Format = s
		return nil
	})
}

//
// WithColor activates the colorization of outputs
// auto => yes if os.Stdout is a tty
// yes
// no
//
func WithColor(s string) Option {
	return optionFunc(func(t *T) error {
		t.Color = s
		return nil
	})
}

//
// WithServer sets the api url.
//
func WithServer(s string) Option {
	return optionFunc(func(t *T) error {
		t.Server = s
		return nil
	})
}

// WithLocalRun sets a function to run if the the action is local
func WithLocalRun(f func() (interface{}, error)) Option {
	return optionFunc(func(t *T) error {
		t.Node.Run = f
		return nil
	})
}

// Options returns the base Action struct
func (t T) Options() action.T {
	return action.T(t.T)
}

func (t T) DoLocal() {
	r := object.NewNode().Do(t.Node)
	human := func() string {
		s := ""
		switch {
		case r.Error != nil:
			log.Error().Msgf("%s", r.Error)
		case r.Panic != nil:
			log.Fatal().Msgf("%s", r.Panic)
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
		return s
	}
	output.Renderer{
		Format:        t.Format,
		Color:         t.Color,
		Data:          r,
		HumanRenderer: human,
	}.Print()
}

// DoAsync uses the agent API to submit a target state to reach via an
// orchestration.
func (t T) DoAsync() {
	c, err := client.New(client.URL(t.Server))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
	req := c.NewPostNodeMonitor()
	req.GlobalExpect = t.Target
	b, err := req.Do()
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
}

// DoRemote posts the action to a peer node agent API, for synchronous
// execution.
func (t T) DoRemote() {
	c, err := client.New(client.URL(t.Server))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
	req := c.NewPostNodeAction()
	req.NodeSelector = t.NodeSelector
	req.Action = t.Action
	req.Options = t.PostFlags
	b, err := req.Do()
	data := &struct {
		Err    string `json:"err"`
		Out    string `json:"out"`
		Status int    `json:"status"`
	}{}
	if err := json.Unmarshal(b, data); err != nil {
		log.Error().Err(err).Msg("")
		return
	}
	fmt.Fprintf(os.Stdout, data.Out)
	fmt.Fprintf(os.Stderr, data.Err)
}

func (t T) Do() {
	action.Do(t)
}
