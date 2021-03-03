package action

import (
	"fmt"
	"os"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
)

type (
	// ObjectAction has the same attributes as Action, but the interface
	// method implementation differ.
	ObjectAction Action
)

// Do is the switch method between local, remote or async mode.
// If Watch is set, end up starting a monitor on the selected objects.
func (t ObjectAction) Do() {
	if t.Local {
		object.NewSelection(t.ObjectSelector).Action(t.Method)
		return
	}
	do(t)
}

// Options returns the base Action struct
func (t ObjectAction) Options() Action {
	return Action(t)
}

// DoAsync uses the agent API to submit a target state to reach via an
// orchestration.
func (t ObjectAction) DoAsync() {
	api, err := client.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
	for _, path := range object.NewSelection(t.ObjectSelector).Expand() {
		req := api.NewPostObjectMonitor()
		req.ObjectSelector = path.String()
		req.GlobalExpect = t.Target
		b, err := req.Do()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
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
func (t ObjectAction) DoRemote() {
	api, err := client.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
	req := api.NewPostObjectAction()
	req.ObjectSelector = t.ObjectSelector
	req.NodeSelector = t.NodeSelector
	req.Action = t.Action
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
