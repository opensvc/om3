package action

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
)

type (
	// NodeAction has the same attributes as Action, but the interface
	// method implementation differ.
	NodeAction struct {
		Node object.NodeAction
		Action
	}
)

// Options returns the base Action struct
func (t NodeAction) options() Action {
	return Action(t.Action)
}

func (t NodeAction) doLocal() {
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
func (t NodeAction) doAsync() {
	c := client.NewConfig()
	c.SetURL(t.Server)
	api, err := c.NewAPI()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
	req := api.NewPostNodeMonitor()
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
func (t NodeAction) doRemote() {
	c := client.NewConfig()
	c.SetURL(t.Server)
	api, err := c.NewAPI()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
	req := api.NewPostNodeAction()
	req.NodeSelector = t.NodeSelector
	req.Action = t.Action.Action
	req.Options = t.Action.PostFlags
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
