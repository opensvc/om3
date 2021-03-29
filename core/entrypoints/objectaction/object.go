package objectaction

import (
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
	// ObjectAction has the same attributes as Action, but the interface
	// method implementation differ.
	T struct {
		action.T
		Object object.Action
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

// New allocates a new client configuration and returns the reference
// so users are not tempted to use client.Config{} dereferenced, which would
// make loadContext useless.
func New(opts ...Option) *T {
	t := &T{}
	for _, opt := range opts {
		_ = opt.apply(t)
	}
	return t
}

//
// WithObjectSelector expands into a selection of objects to execute
// the action on.
//
func WithObjectSelector(s string) Option {
	return optionFunc(func(t *T) error {
		t.ObjectSelector = s
		return nil
	})
}

//
// WithRemoteNodes expands into a selection of nodes to execute the
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
func WithLocalRun(f func(object.Path) (interface{}, error)) Option {
	return optionFunc(func(t *T) error {
		t.Object.Run = f
		return nil
	})
}

// Options returns the base Action struct
func (t T) Options() action.T {
	return action.T(t.T)
}

func (t T) DoLocal() {
	log.Debug().
		Str("format", t.Format).
		Str("selector", t.ObjectSelector).
		Msg("do local object selection action")
	sel := object.NewSelection(
		t.ObjectSelector,
		object.SelectionWithLocal(true),
	)
	rs := sel.Do(t.Object)
	human := func() string {
		s := ""
		for _, r := range rs {
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
		}
		return s
	}
	output.Renderer{
		Format:        t.Format,
		Color:         t.Color,
		Data:          rs,
		HumanRenderer: human,
	}.Print()
}

// DoAsync uses the agent API to submit a target state to reach via an
// orchestration.
func (t T) DoAsync() {
	c, err := client.New(client.URL(t.Server))
	if err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	sel := object.NewSelection(
		t.ObjectSelector,
		object.SelectionWithClient(c),
	)
	for _, path := range sel.Expand() {
		req := c.NewPostObjectMonitor()
		req.ObjectSelector = path.String()
		req.GlobalExpect = t.Target
		b, err := req.Do()
		if err != nil {
			log.Error().Err(err).Msg("")
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
	}
}

// DoRemote posts the action to a peer node agent API, for synchronous
// execution.
func (t T) DoRemote() {
	c, err := client.New(client.URL(t.Server))
	if err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	req := c.NewPostObjectAction()
	req.ObjectSelector = t.ObjectSelector
	req.NodeSelector = t.NodeSelector
	req.Action = t.Action
	req.Options = t.PostFlags
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

func (t T) Do() {
	action.Do(t)
}
