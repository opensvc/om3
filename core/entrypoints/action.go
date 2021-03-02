package entrypoints

import (
	"fmt"
	"os"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/entrypoints/monitor"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
)

// Action switches between local, remote or async mode for a command action
type Action struct {
	ObjectSelector string
	NodeSelector   string
	Local          bool
	Action         string
	Method         string
	Target         string
	Watch          bool
	Format         string
	Color          string
}

// Do is the switch method between local, remote or async mode.
// If Watch is set, end up starting a monitor on the selected objects.
func (t Action) Do() {
	if t.Local {
		// TODO: plug local action
		object.NewSelection(t.ObjectSelector).Action(t.Method)
	} else if t.NodeSelector != "" {
		t.DoRemote()
	} else {
		t.DoAsync()
	}
	if t.Watch {
		m := monitor.New()
		m.SetWatch(true)
		m.SetColor(t.Color)
		m.SetFormat(t.Format)
		m.SetSelector(t.ObjectSelector)
		m.Do()
	}
}

// DoAsync uses the agent API to submit a target state to reach via an
// orchestration.
func (t Action) DoAsync() {
	api, err := client.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
	req := api.NewPostObjectMonitor()
	req.ObjectSelector = t.ObjectSelector
	req.GlobalExpect = t.Target
	b, err := req.Do()
	human := func() string {
		s := fmt.Sprintln(string(b))
		return s
	}
	s := output.Switch(t.Format, t.Color, b, human)
	fmt.Print(s)
}

// DoRemote posts the action to a peer node agent API, for synchronous
// execution.
func (t Action) DoRemote() {
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
	s := output.Switch(t.Format, t.Color, b, human)
	fmt.Print(s)
}
