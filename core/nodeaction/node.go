package nodeaction

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/actionrouter"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
)

type (
	// T has is an actionrouter.T with a node func
	T struct {
		actionrouter.T
		Func func() (interface{}, error)
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{}
	_ = funcopt.Apply(t, opts...)
	return t
}

// WithRemoteNodes expands into a selection of nodes to execute the
// action on.
func WithRemoteNodes(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.NodeSelector = s
		return nil
	})
}

// WithLocal routes the action to the CRM instead of remoting it via
// orchestration or remote execution.
func WithLocal(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Local = v
		return nil
	})
}

// LocalFirst makes actions not explicitely Local nor remoted
// via NodeSelector be treated as local (CRM level).
func LocalFirst() funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.DefaultIsLocal = true
		return nil
	})
}

// WithRemoteAction is the name of the action as passed to the command line
// interface.
func WithRemoteAction(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Action = s
		return nil
	})
}

// WithRemoteOptions is the dataset submited in the POST /{object|node}_action
// api handler to execute the action remotely.
func WithRemoteOptions(m map[string]interface{}) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.PostFlags = m
		return nil
	})
}

// WithAsyncTarget is the node or object state the daemons should orchestrate
// to reach.
func WithAsyncTarget(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Target = s
		return nil
	})
}

// WithAsyncWatch runs a event-driven monitor on the selected objects after
// setting a new target. So the operator can see the orchestration
// unfolding.
func WithAsyncWatch(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
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
	return funcopt.F(func(i interface{}) error {
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
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Color = s
		return nil
	})
}

// WithServer sets the api url.
func WithServer(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Server = s
		return nil
	})
}

// WithLocalRun sets a function to run if the the action is local
func WithLocalRun(f func() (interface{}, error)) funcopt.O {
	return funcopt.F(func(i interface{}) error {
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
		Data:          []actionrouter.Result{r},
		HumanRenderer: human,
		Colorize:      rawconfig.Colorize,
	}.Print()
	if r.Panic != nil {
		return errors.New("")
	}
	if r.Error != nil {
		return errors.New("")
	}
	return r.Error
}

// DoAsync uses the agent API to submit a target state to reach via an
// orchestration.
func (t T) DoAsync() {
	c, err := client.New(client.WithURL(t.Server))
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
		Colorize:      rawconfig.Colorize,
	}.Print()
}

// DoRemote posts the action to a peer node agent API, for synchronous
// execution.
func (t T) DoRemote() error {
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	req := c.NewPostNodeAction()
	req.NodeSelector = t.NodeSelector
	req.Action = t.Action
	req.Options = t.PostFlags
	b, err := req.Do()
	if err != nil {
		return err
	}
	data := &struct {
		Err    string `json:"err"`
		Out    string `json:"out"`
		Status int    `json:"status"`
	}{}
	if err := json.Unmarshal(b, data); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, data.Out)
	fmt.Fprintf(os.Stderr, data.Err)
	return nil
}

func (t T) Do() error {
	return actionrouter.Do(t)
}

func nodeDo(fn func() (interface{}, error)) actionrouter.Result {
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
