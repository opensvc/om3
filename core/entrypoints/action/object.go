package action

import (
	"fmt"
	"os"
	"reflect"

	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
)

type (
	// ObjectAction has the same attributes as Action, but the interface
	// method implementation differ.
	ObjectAction struct {
		Action
		Object object.Action
	}
)

// Options returns the base Action struct
func (t ObjectAction) options() Action {
	return Action(t.Action)
}

func (t ObjectAction) doLocal() {
	log.Debug().
		Str("format", t.Format).
		Str("selector", t.ObjectSelector).
		Msg("do local object selection action")
	sel := object.NewSelection(t.ObjectSelector).SetLocal(true)
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
func (t ObjectAction) doAsync() {
	c, err := client.New(client.URL(t.Server))
	if err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	sel := object.NewSelection(t.ObjectSelector)
	sel.SetClient(c)
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
func (t ObjectAction) doRemote() {
	c, err := client.New(client.URL(t.Server))
	if err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	req := c.NewPostObjectAction()
	req.ObjectSelector = t.ObjectSelector
	req.NodeSelector = t.NodeSelector
	req.Action = t.Action.Action
	req.Options = t.Action.PostFlags
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
