package action

import (
	"fmt"
	"os"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/output"
)

type (
	// NodeAction has the same attributes as Action, but the interface
	// method implementation differ.
	NodeAction Action
)

func (t NodeAction) doLocal() {
	//node.New().Action(t.Method)
}

// Options returns the base Action struct
func (t NodeAction) options() Action {
	return Action(t)
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
