package action

import (
	"fmt"
	"os"
	"reflect"

	log "github.com/sirupsen/logrus"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
)

type (
	// ObjectAction has the same attributes as Action, but the interface
	// method implementation differ.
	ObjectAction Action
)

// Options returns the base Action struct
func (t ObjectAction) options() Action {
	return Action(t)
}

func (t ObjectAction) doLocal() {
	sel := object.NewSelection(t.ObjectSelector).SetLocal(true)
	action := object.Action{
		Method:      t.Method,
		MethodArgs:  t.MethodArgs,
		Lock:        t.Lock,
		LockTimeout: t.LockTimeout,
		LockGroup:   t.LockGroup,
	}
	rs := sel.Do(action)
	human := func() string {
		s := ""
		for _, r := range rs {
			switch {
			case r.Error != nil:
				log.Error(r.Error)
			case r.Panic != nil:
				log.Error(r.Panic)
			case r.Data != nil:
				switch v := r.Data.(type) {
				case string:
					s += fmt.Sprintln(v)
				case []string:
					for _, e := range v {
						s += fmt.Sprintln(e)
					}
				default:
					log.Errorf("unimplemented default renderer for local action result of type %s", reflect.TypeOf(v))
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
	c := client.NewConfig()
	c.SetURL(t.Server)
	api, err := c.NewAPI()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	sel := object.NewSelection(t.ObjectSelector)
	sel.SetAPI(api)
	for _, path := range sel.Expand() {
		req := api.NewPostObjectMonitor()
		req.ObjectSelector = path.String()
		req.GlobalExpect = t.Target
		b, err := req.Do()
		if err != nil {
			log.Error(err)
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
	c := client.NewConfig()
	c.SetURL(t.Server)
	api, err := c.NewAPI()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	req := api.NewPostObjectAction()
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
